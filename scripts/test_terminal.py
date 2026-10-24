#!/usr/bin/env python3
"""PTY regression tests. Build bin/cpc first; requires Python package pyte."""
import codecs
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time

import pyte

BINARY = str(Path(__file__).resolve().parents[1] / "bin/cpc")


class Terminal:
    def __init__(self, directory, args=(), width=80, height=18, language="zh"):
        self.master, slave = pty.openpty()
        self.screen = pyte.Screen(width, height)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.resize(width, height)
        env = dict(os.environ, HOME=directory, LC_ALL="zh_CN.UTF-8" if language == "zh" else "en_US.UTF-8", TERM="xterm-256color")
        self.process = subprocess.Popen([BINARY, "--print", *args], cwd=directory, env=env, stdin=slave, stdout=subprocess.PIPE, stderr=slave)
        os.close(slave)

    def resize(self, width, height):
        self.screen.resize(lines=height, columns=width)
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        if hasattr(self, "process"):
            os.kill(self.process.pid, signal.SIGWINCH)

    def pump(self, duration=0.15):
        end = time.monotonic() + duration
        while time.monotonic() < end:
            if select.select([self.master], [], [], min(0.02, max(0, end - time.monotonic())))[0]:
                try:
                    data = os.read(self.master, 65536)
                except OSError:
                    return
                if b"\x1b[6n" in data:
                    os.write(self.master, b"\x1b[1;1R")
                self.stream.feed(self.decoder.decode(data))

    def text(self):
        return "\n".join(self.screen.display)

    def wait_for(self, text):
        end = time.monotonic() + 6
        while text not in self.text():
            self.pump()
            if time.monotonic() > end:
                raise AssertionError(f"Missing {text!r}:\n{self.text()}")

    def send(self, value):
        os.write(self.master, value.encode() if isinstance(value, str) else value)
        self.pump()

    def finish(self, expected=None, code=0):
        try:
            deadline = time.monotonic() + 5
            while self.process.poll() is None and time.monotonic() < deadline:
                self.pump()
            output, _ = self.process.communicate(timeout=1)
            assert self.process.returncode == code, (self.process.returncode, self.text())
            if expected is not None:
                assert output.decode() == expected + "\n", output
        finally:
            self.close()

    def close(self):
        if self.process.poll() is None:
            self.process.kill()
            self.process.wait()
        if self.master >= 0:
            os.close(self.master)
            self.master = -1


def main():
    with tempfile.TemporaryDirectory(prefix="cpc-terminal-") as root:
        path = Path(root) / ("中文项目" * 12)
        path.mkdir()
        directory = str(path.resolve())
        long_name = "build-" + "frontend-" * 12
        (path / "package.json").write_text(json.dumps({"scripts": {long_name: "vite build " * 80, "dev": "vite"}}))
        for args in (["init", "-b", "preview-main"], ["-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "test"]):
            subprocess.run(["git", *args], cwd=directory, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=directory).decode().strip()
        terminal = Terminal(directory, width=80)
        try:
            terminal.wait_for("preview-main")
            terminal.wait_for(commit[:12])
            for width in (40, 80, 120, 40, 80):
                terminal.resize(width, 18)
                terminal.pump()
                for _ in range(4):
                    terminal.send(b"\x0e\x0e\x10\x1b[B\x1b[A")
                    frame = terminal.text()
                    assert frame.count("复制上下文") == 1, frame
                    assert frame.count("搜索:") == 1, frame
            terminal.send("工作")
            terminal.wait_for("工作目录")
            terminal.send("\r")
            terminal.finish(directory)
        finally:
            terminal.close()
        terminal = Terminal(directory, width=40)
        try:
            terminal.wait_for("项目脚本")
            terminal.send("script\r")
            terminal.wait_for("选择脚本")
            terminal.send(b"\x0e\x10")
            terminal.send("build\r")
            terminal.wait_for("选择复制内容")
            terminal.send("\r")
            terminal.finish(long_name)
        finally:
            terminal.close()
        for format_name, expected in (("name", long_name), ("command", "npm run " + long_name), ("content", "vite build " * 80)):
            output = subprocess.check_output([BINARY, "--print", "--format", format_name, "script", long_name], cwd=directory)
            assert output.decode() == expected + "\n", output
            terminal = Terminal(directory, args=("script", long_name), width=40)
            try:
                terminal.wait_for("选择复制内容")
                terminal.send(format_name + "\r")
                terminal.finish(expected)
            finally:
                terminal.close()
        terminal = Terminal(directory, args=("script",), width=40)
        try:
            terminal.wait_for("选择脚本")
            terminal.send("build\r")
            terminal.wait_for("选择复制内容")
            terminal.send(b"\x1b")
            terminal.wait_for("选择脚本")
            assert "build" in terminal.text()
            terminal.send("\r")
            terminal.wait_for("选择复制内容")
            terminal.send("content\r")
            terminal.finish("vite build " * 80)
        finally:
            terminal.close()
        terminal = Terminal(directory, width=40, language="en")
        try:
            terminal.wait_for(commit[:12])
            terminal.send("commit\r")
            terminal.finish(commit)
        finally:
            terminal.close()
        terminal = Terminal(directory, args=("script",), width=40)
        try:
            terminal.wait_for("选择脚本")
            terminal.send(b"\x03")
            terminal.finish(code=130)
        finally:
            terminal.close()
    with tempfile.TemporaryDirectory(prefix="cpc-projects-") as root:
        for kind, manifest, body, expected in (
            ("go", "go.mod", "module example.com/test\n", "go test ./..."),
            ("rust", "Cargo.toml", "[workspace]\nmembers = []\n", "cargo test"),
        ):
            project = Path(root) / kind
            project.mkdir()
            (project / manifest).write_text(body)
            directory = str(project.resolve())
            output = subprocess.check_output([BINARY, "--print", "--format", "command", "script", "test"], cwd=directory)
            assert output.decode() == expected + "\n", output
            for args in ((), ("script",)):
                terminal = Terminal(directory, args=args, width=40)
                try:
                    if not args:
                        terminal.wait_for("项目脚本")
                        terminal.send("script\r")
                    terminal.wait_for("选择脚本")
                    terminal.send("test\r")
                    terminal.wait_for("选择复制内容")
                    terminal.send(b"\x0e\r")
                    terminal.finish(expected)
                finally:
                    terminal.close()
    with tempfile.TemporaryDirectory(prefix="cpc-managers-") as root:
        for manager, lock in (("pnpm", "pnpm-lock.yaml"), ("yarn", "yarn.lock"), ("bun", "bun.lock"), ("npm", "package-lock.json")):
            project = Path(root) / manager
            project.mkdir()
            (project / ".git").mkdir()
            (project / "package.json").write_text(json.dumps({"scripts": {"build": "vite build"}}))
            (project / lock).write_text("")
            directory = str(project.resolve())
            for copy_format, expected in (("name", "build"), ("command", manager + " run build"), ("content", "vite build")):
                output = subprocess.check_output([BINARY, "--print", "script", "build", "--format", copy_format], cwd=directory)
                assert output.decode() == expected + "\n", output
            terminal = Terminal(directory, args=("script", "build"), width=40)
            try:
                terminal.wait_for(manager + " run build")
                terminal.send("command\r")
                terminal.finish(manager + " run build")
            finally:
                terminal.close()
    print("PASS: 40/80/120 columns, resize, repeated navigation, Chinese search, nested scripts, full path/hash/script values, Go/Rust direct and nested selection, npm/pnpm/yarn/bun detection, cancellation")


if __name__ == "__main__":
    main()
