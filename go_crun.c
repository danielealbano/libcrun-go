// go_crun.c - C helper function implementations for libcrun Go bindings
#include "libcrun/include/go_crun.h"

#include <unistd.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <stdint.h>

// Forward declaration of the Go callback (defined via //export in runtime.go)
extern void goLogCallback(uintptr_t handle, int errno_, const char *msg, int verbosity);

// Global handle for the Go log callback (0 = no callback set)
static uintptr_t go_log_handle = 0;

// C wrapper matching crun_output_handler signature
static void go_crun_log_callback(int errno_, const char *msg, int verbosity, void *arg) {
    (void)arg; // We use the global handle instead
    if (go_log_handle != 0) {
        goLogCallback(go_log_handle, errno_, msg, verbosity);
    }
}

void go_crun_set_log_handler(uintptr_t handle) {
    go_log_handle = handle;
    crun_set_output_handler(go_crun_log_callback, NULL);
}

void go_crun_reset_log_handler(void) {
    go_log_handle = 0;
    // Reset to default stderr handler (not NULL, which would cause SIGSEGV)
    crun_set_output_handler(log_write_to_stderr, NULL);
}

// Log handler that writes structured log entries to a pipe.
// Wire format: [errno:4][verbosity:4][msg_len:4][message:msg_len]
// The arg parameter is the file descriptor (cast to intptr_t).
static void log_write_to_pipe(int errno_, const char *msg, int verbosity, void *arg) {
    int fd = (int)(intptr_t)arg;
    int32_t e = (int32_t)errno_;
    int32_t v = (int32_t)verbosity;
    uint32_t len = (uint32_t)strlen(msg);
    ssize_t ignored __attribute__((unused));
    
    // Write header and message (best effort, ignore errors)
    ignored = write(fd, &e, sizeof(e));
    ignored = write(fd, &v, sizeof(v));
    ignored = write(fd, &len, sizeof(len));
    ignored = write(fd, msg, len);
}

// ---- libcrun error → C string helper ----
char* go_crun_err_to_cstr(libcrun_error_t *err, int *status) {
  if (status) *status = 0;
  if (err == NULL || *err == NULL) return NULL;

  char *p = NULL;
  if (status) *status = (*err)->status;

  if ((*err)->status == 0) {
    p = strdup((*err)->msg);
  } else {
    if (asprintf(&p, "%s: %s", (*err)->msg, strerror((*err)->status)) < 0)
      p = NULL;
  }
  libcrun_error_release(err);
  return p;
}

// ---- RuntimeContext allocation / free ----
libcrun_context_t* go_crun_new_context(void) {
  libcrun_context_t *ctx = (libcrun_context_t*) calloc(1, sizeof(libcrun_context_t));
  if (!ctx) return NULL;
  ctx->fifo_exec_wait_fd = -1;
  return ctx;
}

void go_crun_free_context(libcrun_context_t *ctx) {
  if (!ctx) return;
  free((char*)ctx->state_root);
  free((char*)ctx->id);
  free((char*)ctx->bundle);
  free((char*)ctx->console_socket);
  free((char*)ctx->pid_file);
  free((char*)ctx->notify_socket);
  free((char*)ctx->handler);
  free(ctx);
}

// ---- Container release (mirror Python binding: free container_def) ----
void go_crun_free_container(libcrun_container_t *ctr) {
  if (!ctr) return;
  if (ctr->container_def) {
    free_runtime_spec_schema_config_schema(ctr->container_def);
    ctr->container_def = NULL;
  }
  // libcrun doesn't expose a destructor for the opaque container struct here;
  // Python binding frees only container_def. We do the same.
}

// ---- JSON sinks via open_memstream ----
char* go_crun_state_json(libcrun_context_t *ctx, const char *id, int *out_len, libcrun_error_t *err) {
  char *buf = NULL;
  size_t sz = 0;
  FILE *fp = open_memstream(&buf, &sz);
  if (!fp) { libcrun_make_error(err, errno, "open_memstream failed"); return NULL; }
  int rc = libcrun_container_state(ctx, id, fp, err);
  fclose(fp);
  if (rc < 0) { free(buf); return NULL; }
  if (out_len) *out_len = (int)sz;
  return buf;
}

char* go_crun_spec_json(bool rootless, int *out_len, libcrun_error_t *err) {
  char *buf = NULL;
  size_t sz = 0;
  FILE *fp = open_memstream(&buf, &sz);
  if (!fp) { libcrun_make_error(err, errno, "open_memstream failed"); return NULL; }
  int rc = libcrun_container_spec(rootless, fp, err);
  fclose(fp);
  if (rc < 0) { free(buf); return NULL; }
  if (out_len) *out_len = (int)sz;
  return buf;
}

// ---- List helper -> char** ----
int go_crun_list(const char *state_root, char ***out, int *out_len, libcrun_error_t *err) {
  libcrun_container_list_t *lst = NULL, *it = NULL;
  int rc = libcrun_get_containers_list(&lst, state_root, err);
  if (rc < 0) return rc;

  int n = 0;
  for (it = lst; it; it = it->next) n++;
  char **arr = (char**)calloc((size_t)n, sizeof(char*));
  if (!arr) {
    libcrun_free_containers_list(lst);
    return libcrun_make_error(err, errno, "calloc failed");
  }
  int i = 0;
  for (it = lst; it; it = it->next) arr[i++] = strdup(it->name ? it->name : "");
  libcrun_free_containers_list(lst);
  *out = arr;
  *out_len = n;
  return 0;
}

void go_crun_free_strv(char **v, int n) {
  if (!v) return;
  for (int i = 0; i < n; i++) free(v[i]);
  free(v);
}

// ---- Exec/update: runtime process JSON ----
int go_crun_exec_json(libcrun_context_t *ctx, const char *id, const char *json, libcrun_error_t *err) {
  char errbuf[1024] = {0};
  yajl_val tree = yajl_tree_parse(json, errbuf, sizeof(errbuf));
  if (!tree) return libcrun_make_error(err, 0, "cannot parse the data: `%s`", errbuf);

  parser_error p_err = NULL;
  struct parser_context pctx = { 0, stderr };
  runtime_spec_schema_config_schema_process *proc =
    make_runtime_spec_schema_config_schema_process(tree, &pctx, &p_err);
  yajl_tree_free(tree);

  if (!proc) {
    int rc = libcrun_make_error(err, 0, "cannot parse process: %s", p_err ? p_err : "unknown");
    free(p_err);
    return rc;
  }

  int rc = libcrun_container_exec(ctx, id, proc, err);
  free_runtime_spec_schema_config_schema_process(proc);
  return rc;
}

// ---- Pause/Unpause ----
int go_crun_pause(libcrun_context_t *ctx, const char *id, libcrun_error_t *err) {
  return libcrun_container_pause(ctx, id, err);
}

int go_crun_unpause(libcrun_context_t *ctx, const char *id, libcrun_error_t *err) {
  return libcrun_container_unpause(ctx, id, err);
}

// ---- Kill all processes ----
int go_crun_killall(libcrun_context_t *ctx, const char *id, const char *signal, libcrun_error_t *err) {
  return libcrun_container_killall(ctx, id, signal, err);
}

// ---- Update container resources ----
int go_crun_update(libcrun_context_t *ctx, const char *id, const char *content, size_t len, libcrun_error_t *err) {
  return libcrun_container_update(ctx, id, content, len, err);
}

// ---- Read container status for IsRunning check ----
int go_crun_is_running(const char *state_root, const char *id, libcrun_error_t *err) {
  libcrun_container_status_t status = {0};
  int rc = libcrun_read_container_status(&status, state_root, id, err);
  if (rc < 0) {
    return rc;
  }
  int running = libcrun_is_container_running(&status, err);
  libcrun_free_container_status(&status);
  return running;
}

// ---- Read PIDs ----
int go_crun_read_pids(libcrun_context_t *ctx, const char *id, int recurse, pid_t **out_pids, int *out_len, libcrun_error_t *err) {
  pid_t *pids = NULL;
  int rc = libcrun_container_read_pids(ctx, id, recurse ? true : false, &pids, err);
  if (rc < 0) return rc;
  *out_pids = pids;
  *out_len = rc;
  return 0;
}

void go_crun_free_pids(pid_t *pids) {
  if (pids) free(pids);
}

// ---- Run container with isolated I/O via fork ----
int go_crun_run_with_pipes(
    libcrun_context_t *ctx,
    libcrun_container_t *container,
    unsigned int flags,
    int stdin_fd,
    int stdout_fd,
    int stderr_fd,
    int log_fd,
    pid_t *out_pid,
    libcrun_error_t *err
) {
  // Create a pipe to communicate errors from child to parent
  int error_pipe[2];
  if (pipe(error_pipe) < 0) {
    return libcrun_make_error(err, errno, "pipe failed");
  }

  pid_t pid = fork();
  if (pid < 0) {
    close(error_pipe[0]);
    close(error_pipe[1]);
    return libcrun_make_error(err, errno, "fork failed");
  }

  if (pid == 0) {
    // Child process
    close(error_pipe[0]); // Close read end
    ssize_t ignored __attribute__((unused));

    // Set up log handler for child process.
    // The Go callback is not valid after fork, so we either:
    // - Use log_write_to_pipe if log_fd >= 0 (parent will read from pipe)
    // - Fall back to log_write_to_stderr otherwise
    if (log_fd >= 0) {
      crun_set_output_handler(log_write_to_pipe, (void *)(intptr_t)log_fd);
    } else {
      crun_set_output_handler(log_write_to_stderr, NULL);
    }

    // Redirect stdin
    if (stdin_fd >= 0) {
      if (dup2(stdin_fd, STDIN_FILENO) < 0) {
        int e = errno;
        ignored = write(error_pipe[1], &e, sizeof(e));
        _exit(1);
      }
      close(stdin_fd);
    } else {
      // Redirect stdin to /dev/null
      int null_fd = open("/dev/null", O_RDONLY);
      if (null_fd >= 0) {
        dup2(null_fd, STDIN_FILENO);
        close(null_fd);
      }
    }

    // Redirect stdout
    if (stdout_fd >= 0) {
      if (dup2(stdout_fd, STDOUT_FILENO) < 0) {
        int e = errno;
        ignored = write(error_pipe[1], &e, sizeof(e));
        _exit(1);
      }
      close(stdout_fd);
    }

    // Redirect stderr
    if (stderr_fd >= 0) {
      if (dup2(stderr_fd, STDERR_FILENO) < 0) {
        int e = errno;
        ignored = write(error_pipe[1], &e, sizeof(e));
        _exit(1);
      }
      close(stderr_fd);
    }

    // Signal success to parent (write 0)
    int zero = 0;
    ignored = write(error_pipe[1], &zero, sizeof(zero));
    close(error_pipe[1]);

    // Run the container
    libcrun_error_t child_err = NULL;
    int rc = libcrun_container_run(ctx, container, flags, &child_err);
    if (child_err) {
      libcrun_error_release(&child_err);
    }
    // Exit with the container's exit code (rc is the exit status from libcrun)
    _exit(rc < 0 ? 1 : rc);
  }

  // Parent process
  close(error_pipe[1]); // Close write end

  // NOTE: Do NOT close stdin_fd/stdout_fd/stderr_fd here.
  // Go owns these file descriptors and will close them via os.File.Close().
  // Closing them here would cause double-close issues in concurrent scenarios.

  // Check if child setup succeeded
  int child_errno = 0;
  ssize_t n = read(error_pipe[0], &child_errno, sizeof(child_errno));
  close(error_pipe[0]);

  if (n != sizeof(child_errno)) {
    // Child died before writing
    waitpid(pid, NULL, 0);
    return libcrun_make_error(err, 0, "child process failed unexpectedly");
  }

  if (child_errno != 0) {
    // Child failed during setup
    waitpid(pid, NULL, 0);
    return libcrun_make_error(err, child_errno, "child process setup failed");
  }

  *out_pid = pid;
  return 0;
}

// ---- Wait for forked container child ----
int go_crun_wait(pid_t pid, int *exit_code, libcrun_error_t *err) {
  int status;
  pid_t ret;

  do {
    ret = waitpid(pid, &status, 0);
  } while (ret < 0 && errno == EINTR);

  if (ret < 0) {
    return libcrun_make_error(err, errno, "waitpid failed");
  }

  if (WIFEXITED(status)) {
    *exit_code = WEXITSTATUS(status);
  } else if (WIFSIGNALED(status)) {
    *exit_code = 128 + WTERMSIG(status);
  } else {
    *exit_code = -1;
  }

  return 0;
}

