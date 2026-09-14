from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any


class WorkerError(RuntimeError):
    pass


class WorkerClient:
    def __init__(self, executable: str | Path):
        self.process = subprocess.Popen(
            [str(Path(executable).resolve())], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, bufsize=1,
        )

    def request(self, method: str, params: dict[str, Any] | None = None) -> list[dict[str, Any]]:
        if self.process.stdin is None or self.process.stdout is None:
            raise WorkerError("worker pipes are unavailable")
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "id": "1", "method": method, "params": params or {}}) + "\n")
        self.process.stdin.flush()
        messages = []
        for line in self.process.stdout:
            message = json.loads(line)
            messages.append(message)
            if message.get("id") == "1" and ("result" in message or "error" in message):
                if "error" in message:
                    raise WorkerError(message["error"]["message"])
                return messages
        raise WorkerError("worker exited before responding")

    def close(self) -> None:
        if self.process.stdin:
            self.process.stdin.close()
        self.process.wait(timeout=5)
