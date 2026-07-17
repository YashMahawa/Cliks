import ApplicationServices
import Foundation

// This process is intentionally tiny. It never asks CoreGraphics for key
// values, characters, modifiers, coordinates, window names, or application
// names. The parent receives only three fixed tokens.
func emit(_ token: String) {
    FileHandle.standardOutput.write(Data((token + "\n").utf8))
}

let mask: CGEventMask = (CGEventMask(1) << CGEventType.keyDown.rawValue)
    | (CGEventMask(1) << CGEventType.leftMouseDown.rawValue)
    | (CGEventMask(1) << CGEventType.rightMouseDown.rawValue)

if !CGPreflightListenEventAccess() {
    CGRequestListenEventAccess()
}

let callback: CGEventTapCallBack = { _, type, event, _ in
    switch type {
    case .keyDown: emit("k")
    case .leftMouseDown: emit("l")
    case .rightMouseDown: emit("r")
    default: break
    }
    return Unmanaged.passUnretained(event)
}

guard let tap = CGEvent.tapCreate(
    tap: .cgSessionEventTap,
    place: .headInsertEventTap,
    options: .listenOnly,
    eventsOfInterest: mask,
    callback: callback,
    userInfo: nil
) else {
    fputs("Cliks Capture needs Input Monitoring permission.\n", stderr)
    exit(2)
}

let source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0)
CFRunLoopAddSource(CFRunLoopGetCurrent(), source, .commonModes)
CGEvent.tapEnable(tap: tap, enable: true)
CFRunLoopRun()
