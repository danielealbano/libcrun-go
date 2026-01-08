/* config.h.  Generated from config.h.in by configure.  */
/* config.h.in.  Generated from configure.ac by autoheader.  */

/* Define if CRIU_CONFIG_FILE is available */
#define CRIU_CONFIG_FILE 1

/* Define if CRIU join NS support is available */
#define CRIU_JOIN_NS_SUPPORT 1

/* Define if CRIU_NETWORK_LOCK_SKIP is available */
#define CRIU_NETWORK_LOCK_SKIP_SUPPORT 1

/* Define if CRIU pre-dump support is available */
#define CRIU_PRE_DUMP_SUPPORT 1

/* Define if shared libraries are enabled */
/* #undef DYNLOAD_LIBCRUN */

/* Define to 1 if the system has the type `atomic_int'. */
#define HAVE_ATOMIC_INT 1

/* Define if libcap is available */
#define HAVE_CAP 1

/* Define to 1 if you have the `copy_file_range' function. */
#define HAVE_COPY_FILE_RANGE 1

/* Define if code coverage is enabled */
/* #undef HAVE_COVERAGE */

/* Define if CRIU is available */
#define HAVE_CRIU 1

/* Define to 1 if you have the <dlfcn.h> header file. */
#define HAVE_DLFCN_H 1

/* Define if DLOPEN is available */
#define HAVE_DLOPEN 1

/* Define to 1 if you have the `eaccess' function. */
#define HAVE_EACCESS 1

/* Define if eBPF is available */
#define HAVE_EBPF 1

/* Define if error.h is usable */
#define HAVE_ERROR_H 1

/* Define to 1 if you have the `fgetpwent_r' function. */
#define HAVE_FGETPWENT_R 1

/* Define to 1 if you have the `fgetxattr' function. */
#define HAVE_FGETXATTR 1

/* Define if FSCONFIG_CMD_CREATE is available in linux/mount.h */
#define HAVE_FSCONFIG_CMD_CREATE_LINUX_MOUNT_H 1

/* Define if FSCONFIG_CMD_CREATE is available in sys/mount.h */
#define HAVE_FSCONFIG_CMD_CREATE_SYS_MOUNT_H 1

/* Define to 1 if you have the `hsearch_r' function. */
#define HAVE_HSEARCH_R 1

/* Define to 1 if you have the <inttypes.h> header file. */
#define HAVE_INTTYPES_H 1

/* Define to 1 if you have the `issetugid' function. */
/* #undef HAVE_ISSETUGID */

/* Define to 1 if you have the <lauxlib.h> header file. */
/* #undef HAVE_LAUXLIB_H */

/* Define if libkrun is available */
/* #undef HAVE_LIBKRUN */

/* Define to 1 if you have the <libkrun.h> header file. */
/* #undef HAVE_LIBKRUN_H */

/* Define to 1 if you have the <linux/bpf.h> header file. */
#define HAVE_LINUX_BPF_H 1

/* Define to 1 if you have the <linux/ioprio.h> header file. */
#define HAVE_LINUX_IOPRIO_H 1

/* Define to 1 if you have the <linux/openat2.h> header file. */
#define HAVE_LINUX_OPENAT2_H 1

/* Define if log2 is available */
#define HAVE_LOG2 1

/* Define to 1 if you have the <luaconf.h> header file. */
/* #undef HAVE_LUACONF_H */

/* Define to 1 if you have the <lualib.h> header file. */
/* #undef HAVE_LUALIB_H */

/* Define to 1 if you have the <lua.h> header file. */
/* #undef HAVE_LUA_H */

/* Define to 1 if you have the `memfd_create' function. */
#define HAVE_MEMFD_CREATE 1

/* Define if mono is available */
/* #undef HAVE_MONO */

/* Define to 1 if you have the <mono/metadata/environment.h> header file. */
/* #undef HAVE_MONO_METADATA_ENVIRONMENT_H */

/* Define to 1 if you have the `sd_notify_barrier' function. */
#define HAVE_SD_NOTIFY_BARRIER 1

/* Define if seccomp is available */
#define HAVE_SECCOMP 1

/* Define if SECCOMP_GET_NOTIF_SIZES is available */
#define HAVE_SECCOMP_GET_NOTIF_SIZES 1

/* Define to 1 if you have the <seccomp.h> header file. */
#define HAVE_SECCOMP_H 1

/* Define if spin is available */
#define HAVE_SPIN 1

/* Define to 1 if you have the `statx' function. */
#define HAVE_STATX 1

/* Define to 1 if you have the <stdatomic.h> header file. */
#define HAVE_STDATOMIC_H 1

/* Define to 1 if you have the <stdint.h> header file. */
#define HAVE_STDINT_H 1

/* Define to 1 if you have the <stdio.h> header file. */
#define HAVE_STDIO_H 1

/* Define to 1 if you have the <stdlib.h> header file. */
#define HAVE_STDLIB_H 1

/* Define to 1 if you have the <strings.h> header file. */
#define HAVE_STRINGS_H 1

/* Define to 1 if you have the <string.h> header file. */
#define HAVE_STRING_H 1

/* Define if libsystemd is available */
#define HAVE_SYSTEMD 1

/* Define to 1 if you have the <systemd/sd-bus.h> header file. */
#define HAVE_SYSTEMD_SD_BUS_H 1

/* Define to 1 if you have the <sys/capability.h> header file. */
#define HAVE_SYS_CAPABILITY_H 1

/* Define to 1 if you have the <sys/stat.h> header file. */
#define HAVE_SYS_STAT_H 1

/* Define to 1 if you have the <sys/types.h> header file. */
#define HAVE_SYS_TYPES_H 1

/* Define to 1 if you have the <unistd.h> header file. */
#define HAVE_UNISTD_H 1

/* Define if WAMR is available */
/* #undef HAVE_WAMR */

/* Define if WasmEdge is available */
/* #undef HAVE_WASMEDGE */

/* Define to 1 if you have the <wasmedge/wasmedge.h> header file. */
/* #undef HAVE_WASMEDGE_WASMEDGE_H */

/* Define if wasmer is available */
/* #undef HAVE_WASMER */

/* Define to 1 if you have the <wasmer.h> header file. */
/* #undef HAVE_WASMER_H */

/* Define if wasmtime is available */
/* #undef HAVE_WASMTIME */

/* Define to 1 if you have the <wasmtime.h> header file. */
/* #undef HAVE_WASMTIME_H */

/* Define to 1 if you have the <wasm_export.h> header file. */
/* #undef HAVE_WASM_EXPORT_H */

/* Define if libyajl is available */
/* #undef HAVE_YAJL */

/* LIBCRUN_PUBLIC */
#define LIBCRUN_PUBLIC __attribute__((visibility("default"))) extern

/* Define to the sub-directory where libtool stores uninstalled libraries. */
#define LT_OBJDIR ".libs/"

/* Name of package */
#define PACKAGE "crun"

/* Define to the address where bug reports for this package should be sent. */
#define PACKAGE_BUGREPORT "giuseppe@scrivano.org"

/* Define to the full name of this package. */
#define PACKAGE_NAME "crun"

/* Define to the full name and version of this package. */
#define PACKAGE_STRING "crun 1.26-dirty"

/* Define to the one symbol short name of this package. */
#define PACKAGE_TARNAME "crun"

/* Define to the home page for this package. */
#define PACKAGE_URL ""

/* Define to the version of this package. */
#define PACKAGE_VERSION "1.26-dirty"

/* Define if seccomp_arch_resolve_name is available */
#define SECCOMP_ARCH_RESOLVE_NAME 1

/* Define if shared libraries are enabled */
/* #undef SHARED_LIBCRUN */

/* Define to 1 if all of the C90 standard headers exist (not just the ones
   required in a freestanding environment). This macro is provided for
   backward compatibility; new code need not use it. */
#define STDC_HEADERS 1

/* Version number of package */
#define VERSION "1.26-dirty"
