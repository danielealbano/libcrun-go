// go_crun.h - C helper function declarations for libcrun Go bindings
#ifndef GO_CRUN_H
#define GO_CRUN_H

#define _GNU_SOURCE
#include <stdlib.h>
#include <stdio.h>
#include <stdbool.h>
#include <string.h>
#include <errno.h>
#include <sys/types.h>

#include <libcrun/container.h>
#include <libcrun/status.h>
#include <libcrun/utils.h>
#include <libcrun/error.h>

#include <yajl/yajl_tree.h>

#include <ocispec/runtime_spec_schema_config_schema.h>

// Error handling: convert libcrun error to C string
char* go_crun_err_to_cstr(libcrun_error_t *err, int *status);

// RuntimeContext allocation / free
libcrun_context_t* go_crun_new_context(void);
void go_crun_free_context(libcrun_context_t *ctx);

// Container release (mirror Python binding: free container_def)
void go_crun_free_container(libcrun_container_t *ctr);

// JSON sinks via open_memstream
char* go_crun_state_json(libcrun_context_t *ctx, const char *id, int *out_len, libcrun_error_t *err);
char* go_crun_spec_json(bool rootless, int *out_len, libcrun_error_t *err);

// Container list helpers
int go_crun_list(const char *state_root, char ***out, int *out_len, libcrun_error_t *err);
void go_crun_free_strv(char **v, int n);

// Exec with runtime process JSON
int go_crun_exec_json(libcrun_context_t *ctx, const char *id, const char *json, libcrun_error_t *err);

// Pause/Unpause
int go_crun_pause(libcrun_context_t *ctx, const char *id, libcrun_error_t *err);
int go_crun_unpause(libcrun_context_t *ctx, const char *id, libcrun_error_t *err);

// Kill all processes
int go_crun_killall(libcrun_context_t *ctx, const char *id, const char *signal, libcrun_error_t *err);

// Update container resources
int go_crun_update(libcrun_context_t *ctx, const char *id, const char *content, size_t len, libcrun_error_t *err);

// Check if container is running
int go_crun_is_running(const char *state_root, const char *id, libcrun_error_t *err);

// Read PIDs
int go_crun_read_pids(libcrun_context_t *ctx, const char *id, int recurse, pid_t **out_pids, int *out_len, libcrun_error_t *err);
void go_crun_free_pids(pid_t *pids);

// Run container with isolated I/O via fork
// stdin_fd, stdout_fd, stderr_fd: pipe fds (-1 = use /dev/null for stdin, inherit for stdout/stderr)
// log_fd: write end of log pipe (-1 = use stderr for logs)
// out_pid: receives the forked child PID for later waitpid
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
);

// Wait for forked container child process
int go_crun_wait(pid_t pid, int *exit_code, libcrun_error_t *err);

// Logging callback support - allows Go to receive libcrun logs
// handle: opaque pointer from cgo.Handle for Go callback routing
void go_crun_set_log_handler(uintptr_t handle);

// Reset log handler to default (stderr)
void go_crun_reset_log_handler(void);

#endif // GO_CRUN_H

