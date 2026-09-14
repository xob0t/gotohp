from __future__ import annotations
import json
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

class WorkerError(RuntimeError):
    pass
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
        # The worker reserves stderr for diagnostics.  We do not consume it
        # here, so do not pipe it: an undrained stderr pipe can block uploads.
        self.process = subprocess.Popen(
            [str(Path(executable).resolve())],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            bufsize=1,
        )
        self._next_id = 0
    def _request(self, method: str, params: dict[str, Any] | None = None, on_event: Callable[[dict[str, Any]], None] | None = None) -> list[dict[str, Any]]:
        if self.process.stdin is None or self.process.stdout is None: raise WorkerError("worker pipes are unavailable")
        if self.process.poll() is not None:
            raise WorkerError(f"worker exited with status {self.process.returncode}")
        self._next_id += 1; request_id = str(self._next_id)
        try:
            self.process.stdin.write(json.dumps({"jsonrpc":"2.0","id":request_id,"method":method,"params": (params or {})})+"\n")
            self.process.stdin.flush()
        except (BrokenPipeError, OSError) as exc:
            raise WorkerError("worker input pipe is closed") from exc
        messages=[]
        for line in self.process.stdout:
            try:
                msg = json.loads(line)
            except json.JSONDecodeError as exc:
                raise WorkerError("worker returned invalid JSON") from exc
            messages.append(msg)
            if msg.get("method") and on_event: on_event(msg)
            if "error" in msg and msg.get("id") in (None, request_id):
                raise WorkerError(msg["error"].get("message", "worker request failed"))
            if msg.get("id")==request_id and "result" in msg:
                return messages
        raise WorkerError("worker exited before responding")
    def accounts(self) -> list[Account]:
        result=self._request("accounts.list")[0]["result"]
        return [Account(a["email"], a["email"] == result.get("selected", "")) for a in result.get("accounts",[])]
    def add_credentials(self, auth: str) -> None:
        self._request("credentials.add", {"auth": auth})

    def remove_account(self, email: str) -> None:
        self._request("credentials.remove", {"email": email})

    def select_account(self, email: str) -> None:
        self._request("credentials.select", {"email": email})

    def upload(self, path: str | Path, *, account: str = "", proxy: str = "", saver: bool = False, use_quota: bool = False, recursive: bool = False, threads: int = 3, force: bool = False, delete: bool = False, disable_filter: bool = False, date_from_filename: bool = False, exclude: str = "", album: str = "", pair_live_photos: bool = False, upload_incomplete_live_photos: bool = False, update_existing_photos_to_live: bool = False, ignore_apple_metadata: bool = False, on_progress: Callable[[dict[str, Any]], None] | None = None) -> UploadResult:
        files = []
        options = {"Api": {"Account": account, "Proxy": proxy, "Saver": saver, "UseQuota": use_quota}, "Recursive": recursive, "Threads": threads, "ForceUpload": force, "DeleteFromHost": delete, "DisableUnsupportedFilesFilter": disable_filter, "SetDateFromFilename": date_from_filename, "ExcludePattern": exclude, "AlbumName": "" if album.upper() == "AUTO" else album, "AlbumAutoMode": album.upper() == "AUTO", "PairLivePhotos": pair_live_photos, "SkipIncompleteLivePhotos": not upload_incomplete_live_photos, "UpdateExistingPhotosToLive": update_existing_photos_to_live, "IgnoreAppleMetadata": ignore_apple_metadata}
        messages = self._request("upload", {"paths": [str(path)], "options": options}, on_progress)
        for msg in messages:
            if msg.get("method") == "fileResult":
                result = msg["params"]
                files.append(UploadFile(result.get("Path", ""), result.get("MediaKey", ""), result.get("Skipped", False), result.get("ErrorMessage", "")))
        return UploadResult(files)
    def close(self):
        if self.process.poll() is not None:
            return
        if self.process.stdin:
            self.process.stdin.close()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.terminate()
            try:
                self.process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
    def __enter__(self): return self
    def __exit__(self, *_): self.close()



