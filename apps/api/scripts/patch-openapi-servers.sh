#!/usr/bin/env bash
# Patch OpenAPI servers block for docs-site consumption.
set -euo pipefail

TARGET="${1:?usage: patch-openapi-servers.sh <openapi.json>}"

python3 - "$TARGET" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    spec = json.load(f)

spec["servers"] = [
    {
        "url": "https://{your-orkai-host}/api/v1",
        "description": "Your control plane instance",
    }
]

with open(path, "w", encoding="utf-8") as f:
    json.dump(spec, f, indent=2)
    f.write("\n")
PY
