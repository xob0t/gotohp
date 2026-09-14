import json
import subprocess
import sys
from pathlib import Path


def test_worker_unknown_method():
    exe = Path(__file__).parents[3] / "bin" / "gotohp-worker.exe"
    p = subprocess.run([str(exe)], input=json.dumps({"jsonrpc":"2.0","id":"1","method":"nope"})+"\n", text=True, capture_output=True, check=True)
    msg = json.loads(p.stdout)
    assert msg["error"]["code"] == "method_not_found"
