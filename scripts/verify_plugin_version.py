#!/usr/bin/env python3

import argparse
import ctypes
import json
from pathlib import Path


class Buffer(ctypes.Structure):
    _fields_ = [("ptr", ctypes.c_void_p), ("len", ctypes.c_size_t)]


class PluginAPI(ctypes.Structure):
    _fields_ = [
        ("abi_version", ctypes.c_uint32),
        ("call", ctypes.c_void_p),
        ("free_buffer", ctypes.c_void_p),
        ("shutdown", ctypes.c_void_p),
    ]


def registered_version(library_path: Path) -> str:
    library = ctypes.CDLL(str(library_path))
    api = PluginAPI()

    init = library.cliproxy_plugin_init
    init.argtypes = [ctypes.c_void_p, ctypes.POINTER(PluginAPI)]
    init.restype = ctypes.c_int
    result = init(None, ctypes.byref(api))
    if result != 0:
        raise RuntimeError(f"cliproxy_plugin_init returned {result}")
    if not api.call or not api.free_buffer:
        raise RuntimeError("plugin function table is incomplete")

    call = ctypes.CFUNCTYPE(
        ctypes.c_int,
        ctypes.c_char_p,
        ctypes.c_void_p,
        ctypes.c_size_t,
        ctypes.POINTER(Buffer),
    )(api.call)
    free_buffer = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_size_t)(api.free_buffer)
    shutdown = ctypes.CFUNCTYPE(None)(api.shutdown) if api.shutdown else None

    response = Buffer()
    try:
        result = call(b"plugin.register", None, 0, ctypes.byref(response))
        if result != 0:
            raise RuntimeError(f"plugin.register returned {result}")
        payload = ctypes.string_at(response.ptr, response.len)
        envelope = json.loads(payload)
        if not envelope.get("ok"):
            raise RuntimeError(f"plugin.register failed: {envelope.get('error')}")
        metadata = envelope["result"]["metadata"]
        return str(metadata.get("Version") or metadata.get("version") or "").strip()
    finally:
        if response.ptr:
            free_buffer(response.ptr, response.len)
        if shutdown:
            shutdown()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("library", type=Path)
    parser.add_argument("expected_version")
    args = parser.parse_args()

    actual = registered_version(args.library.resolve())
    if actual != args.expected_version:
        raise SystemExit(
            f"{args.library}: registered version {actual!r}, "
            f"expected {args.expected_version!r}"
        )
    print(f"{args.library}: registered version {actual}")


if __name__ == "__main__":
    main()
