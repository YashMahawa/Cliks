#ifndef CLIKS_CAPTURE_NATIVE_DARWIN_H
#define CLIKS_CAPTURE_NATIVE_DARWIN_H

#include <stdint.h>

typedef struct CliksEventTap CliksEventTap;

int cliks_event_tap_access_allowed(void);
int cliks_event_tap_request_access(void);
CliksEventTap *cliks_event_tap_create(uintptr_t token);
void cliks_event_tap_run(CliksEventTap *handle);
void cliks_event_tap_stop(CliksEventTap *handle);
void cliks_event_tap_destroy(CliksEventTap *handle);

#endif
