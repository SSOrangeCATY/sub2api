import hashlib
import importlib.util
import io
import json
from pathlib import Path
import stat
import unittest
import zipfile

spec = importlib.util.spec_from_file_location(
    "production_identity", Path(__file__).with_name("production_identity.py"))
identity = importlib.util.module_from_spec(spec)
spec.loader.exec_module(identity)

SHA = "a" * 40
REPO = "Example/Service"
RUN_ID = 135
ENV = {
    "GITHUB_REF": "refs/heads/production", "GITHUB_REPOSITORY": REPO,
    "GITHUB_SHA": SHA, "GITHUB_RUN_ID": str(RUN_ID), "GITHUB_RUN_ATTEMPT": "2",
    "IMAGE_TAG": f"ghcr.io/example/service:production-{SHA}",
    "IMAGE_DIGEST": "sha256:" + "1" * 64,
}


def fixture(document_changes=None, run_changes=None, artifact_changes=None, filename=None):
    document = identity.build_identity(ENV)
    document.update(document_changes or {})
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", zipfile.ZIP_DEFLATED) as package:
        package.writestr(filename or identity.IDENTITY_FILE, json.dumps(document))
    archive = stream.getvalue()
    run = {
        "id": RUN_ID, "status": "completed", "conclusion": "success",
        "head_sha": SHA, "head_branch": "production", "event": "workflow_dispatch",
        "path": identity.WORKFLOW_PATH, "run_attempt": 2,
        "repository": {"full_name": REPO}, "head_repository": {"full_name": REPO},
    }
    run.update(run_changes or {})
    artifact = {
        "id": 999, "name": f"production-image-identity-{SHA}-2",
        "expired": False, "size_in_bytes": len(archive),
        "digest": "sha256:" + hashlib.sha256(archive).hexdigest(),
        "workflow_run": {"id": RUN_ID, "head_sha": SHA, "head_branch": "production"},
    }
    artifact.update(artifact_changes or {})
    paths = {
        f"repos/example/service/actions/runs/{RUN_ID}": json.dumps(run).encode(),
        f"repos/example/service/actions/runs/{RUN_ID}/artifacts?per_page=100":
            json.dumps({"artifacts": [artifact]}).encode(),
        "repos/example/service/actions/artifacts/999/zip": archive,
    }
    return paths


class ProductionIdentityTest(unittest.TestCase):
    def resolve(self, paths):
        return identity.resolve_identity(REPO, SHA, RUN_ID, paths.__getitem__)

    def test_success_returns_the_digest_from_the_selected_build_artifact(self):
        result = self.resolve(fixture())
        self.assertEqual(result["image"], "ghcr.io/example/service@" + ENV["IMAGE_DIGEST"])
        self.assertEqual(result["build_workflow_run_id"], RUN_ID)
        self.assertEqual(result["build_workflow_run_attempt"], 2)
        self.assertEqual(result["artifact_id"], 999)

    def test_producer_rejects_other_branches_and_unbound_tags(self):
        for change in ({"GITHUB_REF": "refs/heads/custom/main"},
                       {"IMAGE_TAG": "ghcr.io/example/service:latest"},
                       {"GITHUB_SHA": "short"}, {"IMAGE_DIGEST": "missing"}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                identity.build_identity({**ENV, **change})

    def test_other_run_sources_and_unsuccessful_runs_are_rejected(self):
        for change in ({"head_sha": "b" * 40}, {"head_branch": "custom/main"},
                       {"event": "push"}, {"path": ".github/workflows/release.yml"},
                       {"status": "in_progress"}, {"conclusion": "failure"},
                       {"head_repository": {"full_name": "other/fork"}}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.resolve(fixture(run_changes=change))

    def test_identity_is_bound_to_repository_sha_attempt_tag_and_platform(self):
        for change in ({"repository": "other/service"}, {"production_sha": "b" * 40},
                       {"run_id": RUN_ID + 1}, {"run_attempt": 1}, {"run_id": True},
                       {"tag": "ghcr.io/example/service:latest"},
                       {"platform": "linux/arm64"}, {"schema_version": True},
                       {"image_digest": "sha256:invalid"}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.resolve(fixture(document_changes=change))

    def test_expired_foreign_and_oversized_artifacts_are_rejected(self):
        foreign = {"id": RUN_ID + 1, "head_sha": SHA, "head_branch": "production"}
        for change in ({"expired": True}, {"size_in_bytes": 2 * 1024 * 1024},
                       {"workflow_run": foreign}, {"id": True}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.resolve(fixture(artifact_changes=change))

    def test_replaced_archive_bytes_fail_the_github_artifact_checksum(self):
        paths = fixture()
        paths["repos/example/service/actions/artifacts/999/zip"] += b"changed"
        with self.assertRaisesRegex(ValueError, "checksum"):
            self.resolve(paths)

    def test_archive_members_cannot_escape_or_substitute_the_manifest(self):
        for name in ("../" + identity.IDENTITY_FILE, "different.json"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.resolve(fixture(filename=name))

    def test_missing_or_ambiguous_identity_artifacts_are_rejected(self):
        for duplicate in (False, True):
            paths = fixture()
            key = f"repos/example/service/actions/runs/{RUN_ID}/artifacts?per_page=100"
            data = json.loads(paths[key])
            data["artifacts"] = data["artifacts"] * 2 if duplicate else []
            paths[key] = json.dumps(data).encode()
            with self.subTest(duplicate=duplicate), self.assertRaises(ValueError):
                self.resolve(paths)

    def test_invalid_repository_and_short_sha_are_rejected_before_api_access(self):
        def forbidden(_endpoint):
            self.fail("Invalid input reached the API")
        for repository, sha in (("../repo", SHA), ("owner/..", SHA),
                                (REPO, "short"), ("https://host/repo", SHA)):
            with self.subTest(repository=repository, sha=sha), self.assertRaises(ValueError):
                identity.resolve_identity(repository, sha, RUN_ID, forbidden)

    def test_workflow_records_the_build_step_digest_and_attempt(self):
        root = Path(__file__).resolve().parents[2]
        workflow = (root / ".github/workflows/production-image.yml").read_text()
        self.assertIn("id: image", workflow)
        self.assertIn("IMAGE_DIGEST: " + "$" + "{{ steps.image.outputs.digest }}", workflow)
        self.assertIn("production-image-identity-" + "$" + "{{ github.sha }}-" +
                      "$" + "{{ github.run_attempt }}", workflow)

    def with_archive(self, paths, archive):
        key = f"repos/example/service/actions/runs/{RUN_ID}/artifacts?per_page=100"
        data = json.loads(paths[key])
        data["artifacts"][0]["digest"] = "sha256:" + hashlib.sha256(archive).hexdigest()
        data["artifacts"][0]["size_in_bytes"] = len(archive)
        paths[key] = json.dumps(data).encode()
        paths["repos/example/service/actions/artifacts/999/zip"] = archive
        return paths

    def test_archive_rejects_extra_members_symlinks_and_encryption(self):
        document = json.dumps(identity.build_identity(ENV))
        archives = []
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, "w", zipfile.ZIP_DEFLATED) as package:
            package.writestr(identity.IDENTITY_FILE, document)
            package.writestr("extra.txt", "unexpected")
        archives.append(("extra member", stream.getvalue()))
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, "w", zipfile.ZIP_DEFLATED) as package:
            member = zipfile.ZipInfo(identity.IDENTITY_FILE)
            member.create_system = 3
            member.external_attr = (stat.S_IFLNK | 0o777) * 65536
            package.writestr(member, document)
        archives.append(("symlink", stream.getvalue()))
        paths = fixture()
        encrypted = bytearray(paths["repos/example/service/actions/artifacts/999/zip"])
        encrypted[6] |= 1
        central = encrypted.index(bytes((80, 75, 1, 2)))
        encrypted[central + 8] |= 1
        archives.append(("encrypted", bytes(encrypted)))
        for label, archive in archives:
            with self.subTest(label=label), self.assertRaises(ValueError):
                self.resolve(self.with_archive(fixture(), archive))

    def test_uncompressed_identity_size_boundary(self):
        document = json.dumps(identity.build_identity(ENV))
        for size in (16384, 16385):
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, "w", zipfile.ZIP_DEFLATED) as package:
                package.writestr(identity.IDENTITY_FILE,
                                 document + " " * (size - len(document)))
            paths = self.with_archive(fixture(), stream.getvalue())
            with self.subTest(size=size):
                if size == 16384:
                    self.assertEqual(self.resolve(paths)["artifact_id"], 999)
                else:
                    with self.assertRaises(ValueError):
                        self.resolve(paths)


if __name__ == "__main__":
    unittest.main()
