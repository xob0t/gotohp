import json
import subprocess
import sys
from pathlib import Path


def test_worker_unknown_method():
    exe = Path(__file__).parents[3] / "bin" / "gotohp-worker.exe"
    p = subprocess.run([str(exe)], input=json.dumps({"jsonrpc":"2.0","id":"1","method":"nope"})+"\n", text=True, capture_output=True, check=True)
    msg = json.loads(p.stdout)
    assert msg["error"]["code"] == "method_not_found"
from clients.python.worker_client import Client
import pytest

def test_upload_rejects_nonpositive_threads():
    client = object.__new__(Client)
    with pytest.raises(ValueError, match="positive"):
        client.upload("photo.jpg", threads=0)

def test_upload_accepts_multiple_paths_signature():
    client = object.__new__(Client)
    # Validation occurs before the worker is needed.
    with pytest.raises(ValueError):
        client.upload(["a.jpg", "b.jpg"], threads=-1)
