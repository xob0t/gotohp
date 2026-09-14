from __future__ import annotations
import json, subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

class WorkerError(RuntimeError): pass
@dataclass
class Account:
    email: str
    selected: bool = False
@dataclass
class UploadFile:
    path: str
    media_key: str = ""
    skipped: bool = False
    error: str = ""
@dataclass
class UploadResult:
    files: list[UploadFile]

class Client:
    def __init__(self, executable: str | Path = "bin/gotohp-worker.exe"):
        self.process = subprocess.Popen([str(Path(executable).resolve())], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
        self._next_id = 0
    def _request(self, method: str, params: dict[str, Any] | None = None, on_event: Callable[[dict[str, Any]], None] | None = None) -> list[dict[str, Any]]:
        if self.process.stdin is None or self.process.stdout is None: raise WorkerError("worker pipes are unavailable")
        self._next_id += 1; request_id = str(self._next_id)
        self.process.stdin.write(json.dumps({"jsonrpc":"2.0","id":request_id,"method":method,"params": (params or {})})+"\n"); self.process.stdin.flush()
        messages=[]
        for line in self.process.stdout:
            msg=json.loads(line); messages.append(msg)
            if msg.get("method") and on_event: on_event(msg)
            if msg.get("id")==request_id and ("result" in msg or "error" in msg):
                if "error" in msg: raise WorkerError(msg["error"]["message"])
                return messages
        raise WorkerError("worker exited before responding")
    def accounts(self) -> list[Account]:
        result=self._request("accounts.list")[0]["result"]
        return [Account(a["email"], a["email"] == result.get("selected", "")) for a in result.get("accounts",[])]
    def upload(self, path: str | Path, *, account: str = "", recursive: bool = False, threads: int = 3, on_progress: Callable[[dict[str, Any]], None] | None = None) -> UploadResult:
        files=[]
        for msg in self._request("upload", {"paths":[str(path)],"account":account,"recursive":recursive,"threads":threads}, on_progress):
            if msg.get("method")=="fileResult":
                p=msg["params"]; files.append(UploadFile(p.get("Path", ""),p.get("MediaKey", ""),p.get("Skipped",False),p.get("ErrorMessage", "")))
        return UploadResult(files)
    def close(self):
        if self.process.stdin: self.process.stdin.close()
        self.process.wait(timeout=5)
    def __enter__(self): return self
    def __exit__(self, *_): self.close()



