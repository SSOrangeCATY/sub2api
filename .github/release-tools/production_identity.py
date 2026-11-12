#!/usr/bin/env python3
"""Resolve an immutable production digest from its successful build artifact."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import zipfile

IDENTITY_FILE = "production-image-identity.json"
WORKFLOW_PATH = ".github/workflows/production-image.yml"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def repository_name(value):
    require(isinstance(value, str) and re.fullmatch(
        r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+", value),
        "Invalid repository")
    require(value.split("/")[1] not in (".", ".."), "Invalid repository")
    return value.lower()


def full_sha(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value),
            "Expected a full production SHA")
    return value


def positive_integer(value):
    require(type(value) is int and value > 0, "Expected a positive integer")
    return value


def image_digest(value):
    require(isinstance(value, str) and re.fullmatch(r"sha256:[0-9a-f]{64}", value),
            "Invalid image digest")
    return value


def build_identity(environment):
    require(environment.get("GITHUB_REF") == "refs/heads/production",
            "Production identity requires the production branch")
    repository = repository_name(environment.get("GITHUB_REPOSITORY"))
    sha = full_sha(environment.get("GITHUB_SHA"))
    tag = f"ghcr.io/{repository}:production-{sha}"
    require(environment.get("IMAGE_TAG") == tag, "Unexpected immutable tag")
    return {
        "schema_version": 1, "repository": repository, "production_sha": sha,
        "run_id": positive_integer(int(environment["GITHUB_RUN_ID"])),
        "run_attempt": positive_integer(int(environment["GITHUB_RUN_ATTEMPT"])),
        "tag": tag, "image_digest": image_digest(environment.get("IMAGE_DIGEST")),
        "platform": "linux/amd64",
    }


def validate_run(run, repository, sha, run_id):
    require(isinstance(run, dict), "Invalid workflow metadata")
    require(run.get("id") == run_id and run.get("status") == "completed"
            and run.get("conclusion") == "success", "Build run is not successful")
    require(run.get("head_sha") == sha and run.get("head_branch") == "production"
            and run.get("event") == "workflow_dispatch"
            and run.get("path") == WORKFLOW_PATH, "Build run has the wrong source")
    for key in ("repository", "head_repository"):
        require(repository_name((run.get(key) or {}).get("full_name")) == repository,
                "Build run belongs to another repository")
    return positive_integer(run.get("run_attempt"))


def gh_api(endpoint):
    return subprocess.check_output(["gh", "api", endpoint],
                                   stderr=subprocess.PIPE, timeout=60)


def resolve_identity(repository, sha, run_id, api=gh_api):
    repository = repository_name(repository)
    sha = full_sha(sha)
    positive_integer(run_id)
    run_path = f"repos/{repository}/actions/runs/{run_id}"
    run = json.loads(api(run_path))
    attempt = validate_run(run, repository, sha, run_id)
    name = f"production-image-identity-{sha}-{attempt}"
    listing = json.loads(api(run_path + "/artifacts?per_page=100"))
    require(isinstance(listing, dict) and isinstance(listing.get("artifacts"), list),
            "Invalid build artifact listing")
    matches = [item for item in listing["artifacts"]
               if isinstance(item, dict) and item.get("name") == name]
    require(len(matches) == 1, "Expected one immutable image identity artifact")
    artifact = matches[0]
    artifact_id = positive_integer(artifact.get("id"))
    require(artifact.get("expired") is False, "Build identity artifact expired")
    size = positive_integer(artifact.get("size_in_bytes"))
    require(not size > 1024 * 1024, "Build identity artifact is oversized")
    source = artifact.get("workflow_run") or {}
    require(source.get("id") == run_id and source.get("head_sha") == sha
            and source.get("head_branch") == "production",
            "Artifact belongs to another build run")
    archive = api(f"repos/{repository}/actions/artifacts/{artifact_id}/zip")
    require(isinstance(archive, bytes) and not len(archive) > 1024 * 1024,
            "Invalid build identity archive")
    digest = "sha256:" + hashlib.sha256(archive).hexdigest()
    require(artifact.get("digest") == digest, "Build artifact checksum mismatch")
    with zipfile.ZipFile(io.BytesIO(archive)) as package:
        members = package.infolist()
        require(len(members) == 1, "Unexpected build identity archive members")
        member = members[0]
        require(member.filename == IDENTITY_FILE and not member.is_dir()
                and not stat.S_ISLNK(member.external_attr >> 16)
                and not member.flag_bits & 1 and not member.file_size > 16384,
                "Invalid build identity archive member")
        document = json.loads(package.read(member).decode("utf-8"))
    require(isinstance(document, dict), "Invalid image identity document")
    require(type(document.get("schema_version")) is int
            and document["schema_version"] == 1, "Unsupported image identity schema")
    expected = {
        "repository": repository, "production_sha": sha, "run_id": run_id,
        "run_attempt": attempt, "tag": f"ghcr.io/{repository}:production-{sha}",
        "platform": "linux/amd64",
    }
    for key, value in expected.items():
        require(type(document.get(key)) is type(value) and document[key] == value,
                "Image identity differs from the selected build run")
    image = f"ghcr.io/{repository}@{image_digest(document.get('image_digest'))}"
    return {
        "production_sha": sha, "repository": repository, "tag": expected["tag"],
        "image": image, "platform": "linux/amd64",
        "build_workflow_run_id": run_id, "build_workflow_run_attempt": attempt,
        "artifact_id": artifact_id, "artifact_digest": digest,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    write = commands.add_parser("write")
    write.add_argument("--output", default=IDENTITY_FILE)
    resolve = commands.add_parser("resolve")
    resolve.add_argument("--repository", required=True)
    resolve.add_argument("--sha", required=True)
    resolve.add_argument("--run-id", required=True, type=int)
    resolve.add_argument("--output")
    args = parser.parse_args()
    result = build_identity(os.environ) if args.command == "write" else resolve_identity(
        args.repository, args.sha, args.run_id)
    rendered = json.dumps(result, indent=2) + chr(10)
    if args.output:
        Path(args.output).write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")


if __name__ == "__main__":
    main()
