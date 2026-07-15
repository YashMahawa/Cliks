//go:build darwin

#include "capture_native_darwin.h"

#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <pthread.h>
#include <stdbool.h>
#include <stdlib.h>

extern void cliksCaptureDarwinEvent(uintptr_t token, int kind, int button);

struct CliksEventTap {
    CFMachPortRef tap;
    CFRunLoopSourceRef source;
    CFRunLoopRef run_loop;
    uintptr_t token;
    bool stopped;
    pthread_mutex_t lock;
};

int cliks_event_tap_access_allowed(void) {
    return CGPreflightListenEventAccess() ? 1 : 0;
}

int cliks_event_tap_request_access(void) {
    return CGRequestListenEventAccess() ? 1 : 0;
}

static CGEventRef cliks_event_tap_callback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *user_info) {
    (void)proxy;
    CliksEventTap *handle = (CliksEventTap *)user_info;
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        CGEventTapEnable(handle->tap, true);
        return event;
    }
    if (type == kCGEventKeyDown) {
        cliksCaptureDarwinEvent(handle->token, 1, 0);
    } else if (type == kCGEventLeftMouseDown) {
        cliksCaptureDarwinEvent(handle->token, 2, 1);
    } else if (type == kCGEventRightMouseDown) {
        cliksCaptureDarwinEvent(handle->token, 2, 2);
    }
    return event;
}

CliksEventTap *cliks_event_tap_create(uintptr_t token) {
    CliksEventTap *handle = calloc(1, sizeof(CliksEventTap));
    if (handle == NULL) {
        return NULL;
    }
    pthread_mutex_init(&handle->lock, NULL);
    handle->token = token;
    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) |
                       CGEventMaskBit(kCGEventLeftMouseDown) |
                       CGEventMaskBit(kCGEventRightMouseDown);
    handle->tap = CGEventTapCreate(kCGSessionEventTap,
                                   kCGHeadInsertEventTap,
                                   kCGEventTapOptionListenOnly,
                                   mask,
                                   cliks_event_tap_callback,
                                   handle);
    if (handle->tap == NULL) {
        pthread_mutex_destroy(&handle->lock);
        free(handle);
        return NULL;
    }
    handle->source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, handle->tap, 0);
    if (handle->source == NULL) {
        CFRelease(handle->tap);
        pthread_mutex_destroy(&handle->lock);
        free(handle);
        return NULL;
    }
    return handle;
}

void cliks_event_tap_run(CliksEventTap *handle) {
    if (handle == NULL) {
        return;
    }
    pthread_mutex_lock(&handle->lock);
    handle->run_loop = CFRunLoopGetCurrent();
    CFRetain(handle->run_loop);
    bool stopped = handle->stopped;
    pthread_mutex_unlock(&handle->lock);

    if (!stopped) {
        CFRunLoopAddSource(handle->run_loop, handle->source, kCFRunLoopCommonModes);
        CGEventTapEnable(handle->tap, true);
        CFRunLoopRun();
        CFRunLoopRemoveSource(handle->run_loop, handle->source, kCFRunLoopCommonModes);
    }

    pthread_mutex_lock(&handle->lock);
    CFRunLoopRef loop = handle->run_loop;
    handle->run_loop = NULL;
    pthread_mutex_unlock(&handle->lock);
    if (loop != NULL) {
        CFRelease(loop);
    }
}

void cliks_event_tap_stop(CliksEventTap *handle) {
    if (handle == NULL) {
        return;
    }
    pthread_mutex_lock(&handle->lock);
    handle->stopped = true;
    CFRunLoopRef loop = handle->run_loop;
    if (loop != NULL) {
        CFRetain(loop);
    }
    pthread_mutex_unlock(&handle->lock);
    if (loop != NULL) {
        CFRunLoopStop(loop);
        CFRelease(loop);
    }
}

void cliks_event_tap_destroy(CliksEventTap *handle) {
    if (handle == NULL) {
        return;
    }
    CFRelease(handle->source);
    CFRelease(handle->tap);
    pthread_mutex_destroy(&handle->lock);
    free(handle);
}
