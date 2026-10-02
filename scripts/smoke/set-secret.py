#!/usr/bin/env python3
"""Types the SePay test secret into `stayguard sepay set-secret`, which reads a terminal only (never an argument or a pipe).
usage: set-secret.py <secret> <command...>   (the command is the full `docker compose ... run --rm api sepay set-secret ...`)"""
import os
import pty
import sys

secret, cmd = sys.argv[1], sys.argv[2:]
pid, fd = pty.fork()
if pid == 0:
    os.execvp(cmd[0], cmd)
out = b""
sent = False
while True:
    try:
        chunk = os.read(fd, 4096)
    except OSError:
        break
    if not chunk:
        break
    out += chunk
    if not sent and b"(hidden)" in out:
        os.write(fd, secret.encode() + b"\n")
        sent = True
_, status = os.waitpid(pid, 0)
text = out.decode(errors="replace").replace(secret, "[secret]")
sys.stdout.write(text)
sys.exit(os.waitstatus_to_exitcode(status) if sent else 1)
