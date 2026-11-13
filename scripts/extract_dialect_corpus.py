#!/usr/bin/env python3
"""Extract the dialect round-trip and transpile corpora from upstream tobymao/sqlglot's
dialect test suite into testdata/dialect_identity.jsonl and testdata/transpile_corpus.jsonl.

Why this exists: tests/dialects/test_mysql.py and tests/dialects/test_postgres.py call
Validator.validate_identity(sql, write_sql=None, pretty=False, ...) hundreds of times, including
inside loops and f-strings that expand into many distinct cases at runtime (e.g. the
BIGINT/INT/... UNSIGNED loop in test_mysql.py, the pretty ARRAY[...] case and GRANT/REVOKE lists
in test_postgres.py). A static regex over the test source cannot recover those expanded cases. So
this script monkeypatches Validator.validate_identity and Validator.validate_all to record every
call, then runs the selected dialect test classes under unittest so every call site — including
loop/template expansions — fires for real. The outputs feed the Go parity harnesses
(corpus_test.go and transpile_corpus_test.go); they are not run by `go test` itself.
MySQL/Postgres records carry hand-added cases, so re-extract those dialects deliberately. For the
Presto-family dialects the same-dialect write of validate_all is recorded too.

validate_all(sql, read={...}, write={...}) also carries the cross-dialect oracle: every read pair
asserts parse_one(read_sql, read_dialect).sql(self.dialect) == sql and every write pair asserts
expression.sql(write_dialect) == write_sql (tests/dialects/test_dialect.py:74). Each pair whose
dialects are all registered in the Go port becomes one transpile record
{"read":..,"write":..,"sql":..,"want":..}: parse `sql` under `read`, generate under `write`,
expect `want`. Pairs naming an unregistered dialect and UnsupportedError wants are skipped.

Run manually (after `scripts/fetch-reference.sh`, since .reference/ is gitignored):

    PYTHONPATH=.reference/sqlglot-v30.17.0 python3 scripts/extract_dialect_corpus.py --transpile-only

Use --transpile-only to refresh testdata/transpile_corpus.jsonl while leaving
testdata/dialect_identity.jsonl byte-identical (a plain full run rewrites every identity section
and would drop the hand-added MySQL/Postgres/base records that per-dialect runs preserve).
Use --dialects athena,trino,presto,hive to replace only those dialects in the existing
dialect_identity.jsonl. Without either flag, all dialects (plus the base TestDialect class) are
extracted and both files are fully rewritten. The transpile corpus mixes pairs from every test
class, so a partial --dialects run leaves testdata/transpile_corpus.jsonl unchanged.

Seongjin.
"""

import argparse
import json
import sys
import unittest
from pathlib import Path

REFERENCE_HINT = (
    "Could not import sqlglot. Run scripts/fetch-reference.sh first, then invoke this script as:\n"
    "  PYTHONPATH=.reference/sqlglot-v30.17.0 python3 scripts/extract_dialect_corpus.py"
)

# Upstream gates some validate_identity cases behind
# @unittest.skipUnless(sys.version_info >= (3, 11)) (tests/dialects/test_mysql.py:912). On an
# older interpreter unittest would silently skip them, truncating the corpus, yet still exit 0.
# Refuse to run below 3.11 so the re-runnable extractor cannot quietly drop cases.
if sys.version_info < (3, 11):
    sys.exit(
        f"extract_dialect_corpus.py requires Python >= 3.11 (running {sys.version.split()[0]}); "
        "older interpreters skip the >= 3.11-gated cases and would truncate the corpus."
    )

try:
    import sqlglot  # noqa: F401
except ImportError:
    print(REFERENCE_HINT, file=sys.stderr)
    raise

from tests.dialects.test_dialect import Validator, TestDialect  # noqa: E402
from tests.dialects.test_athena import TestAthena  # noqa: E402
from tests.dialects.test_trino import TestTrino  # noqa: E402
from tests.dialects.test_presto import TestPresto  # noqa: E402
from tests.dialects.test_hive import TestHive
from tests.dialects.test_tsql import TestTSQL  # noqa: E402
from tests.dialects.test_clickhouse import TestClickhouse  # noqa: E402
from tests.dialects.test_starrocks import TestStarrocks  # noqa: E402
from tests.dialects.test_doris import TestDoris  # noqa: E402
from tests.dialects.test_oracle import TestOracle  # noqa: E402
from tests.dialects.test_redshift import TestRedshift  # noqa: E402
from tests.dialects.test_materialize import TestMaterialize  # noqa: E402
from tests.dialects.test_risingwave import TestRisingWave  # noqa: E402
from tests.dialects.test_spark import TestSpark  # noqa: E402
from tests.dialects.test_databricks import TestDatabricks  # noqa: E402
from tests.dialects.test_bigquery import TestBigQuery  # noqa: E402
from tests.dialects.test_duckdb import TestDuckDB  # noqa: E402
from tests.dialects.test_snowflake import TestSnowflake  # noqa: E402
from tests.dialects.test_mysql import TestMySQL  # noqa: E402
from tests.dialects.test_postgres import TestPostgres  # noqa: E402

DIALECTS = ("bigquery", "duckdb", "snowflake", "mysql", "postgres", "athena", "trino", "presto", "hive", "tsql", "clickhouse", "starrocks", "doris", "oracle", "redshift", "materialize", "risingwave", "spark", "databricks")
SAME_DIALECT_WRITES = {"bigquery", "duckdb", "snowflake", "athena", "trino", "presto", "hive", "tsql", "clickhouse", "starrocks", "doris", "oracle", "redshift", "materialize", "risingwave", "spark", "databricks"}

# Every dialect the Go port registers (dialects.GetOrRaise): the 19 named ones plus base "".
# A transpile pair is captured only when BOTH of its dialects are in this set, so the corpus
# never names a dialect the Go harness cannot parse/generate.
REGISTERED = set(DIALECTS) | {""}


def _norm_dialect(dialect):
    """Map a test-class/validate_all dialect key to the Go port's spelling.

    TestDialect carries Validator.dialect = None (base); enum-valued keys (Dialects.X) reduce to
    their value. Everything else is the plain string the test files use.
    """
    if dialect is None:
        return ""
    return str(getattr(dialect, "value", dialect))


captured = []
transpile_captured = []

orig_validate_identity = Validator.validate_identity
orig_validate_all = Validator.validate_all


def patched_validate_identity(
    self, sql, write_sql=None, pretty=False, check_command_warning=False, identify=False
):
    captured.append(
        {
            "dialect": self.dialect,
            "sql": sql,
            "want": write_sql or sql,
            "pretty": bool(pretty),
            "identify": bool(identify),
        }
    )
    try:
        return orig_validate_identity(
            self,
            sql,
            write_sql=write_sql,
            pretty=pretty,
            check_command_warning=check_command_warning,
            identify=identify,
        )
    except Exception:
        return None


def patched_validate_all(self, sql, read=None, write=None, pretty=False, identify=False):
    # Cross-dialect oracle first: record every read/write pair whose dialects are all
    # registered, without running upstream's assertions (the Go harness owns those).
    # read pair: parse_one(read_sql, read_dialect).sql(self.dialect) == sql.
    # write pair: parse_one(sql, self.dialect).sql(write_dialect) == write_sql.
    test_dialect = _norm_dialect(self.dialect)
    if test_dialect in REGISTERED:
        for read_dialect, read_sql in (read or {}).items():
            read_key = _norm_dialect(read_dialect)
            if read_key in REGISTERED and isinstance(read_sql, str) and isinstance(sql, str):
                transpile_captured.append(
                    {
                        "read": read_key,
                        "write": test_dialect,
                        "sql": read_sql,
                        "want": sql,
                        "pretty": bool(pretty),
                        "identify": bool(identify),
                    }
                )
        for write_dialect, write_sql in (write or {}).items():
            write_key = _norm_dialect(write_dialect)
            # UnsupportedError wants assert that generation raises; skip them (not a SQL pair).
            if write_key in REGISTERED and isinstance(write_sql, str) and isinstance(sql, str):
                transpile_captured.append(
                    {
                        "read": test_dialect,
                        "write": write_key,
                        "sql": sql,
                        "want": write_sql,
                        "pretty": bool(pretty),
                        "identify": bool(identify),
                    }
                )

    # The same-dialect write[self.dialect] also lands in the identity corpus for the
    # dialects listed in SAME_DIALECT_WRITES; mysql/postgres keep their hand-curated records.
    # Best effort only, so an unrelated validate_all failure never
    # aborts a test method before its later validate_identity calls fire.
    if self.dialect in SAME_DIALECT_WRITES and write and self.dialect in write:
        want = write[self.dialect]
        if isinstance(want, str):
            captured.append(
                {
                    "dialect": self.dialect,
                    "sql": sql,
                    "want": want,
                    "pretty": bool(pretty),
                    "identify": bool(identify),
                }
            )
    try:
        # Presto checks transpilation logs before later identity cases (test_presto.py:754).
        if self.dialect == "presto":
            return orig_validate_all(self, sql, read, write, pretty, identify)
        self.parse_one(sql)
    except Exception:
        pass
    return None


Validator.validate_identity = patched_validate_identity
Validator.validate_all = patched_validate_all


# Test methods we expect to fail *because of* the monkeypatches above, keyed by
# (class, method). Each entry was verified to make no validate_identity/validate_all
# call AFTER the failing assertion, so the failure cannot truncate a test method
# before its later capture calls fired (losing corpus records silently):
#   - test_array_offset (postgres, clickhouse) asserts on the "Applying array index
#     offset" log lines emitted by the real cross-dialect validate_all transpile,
#     which patched_validate_all neutralizes; the failing assert is the method's last
#     statement (the clickhouse variant holds a single validate_all, already captured).
#   - TestDuckDB.test_array_index is the same log assert, but its validate_all and all
#     five validate_identity calls precede the failing final assertEqual.
#   - TestBigQuery.test_warnings fails at the __exit__ of the assertLogs block whose
#     validate_all/validate_identity calls already ran; the remainder of the method
#     only calls parse() directly (no validate_* capture sites).
# Any failure NOT listed here could have truncated a test method before its later
# validate_identity calls fired (leaving the corpus silently incomplete), so it aborts
# extraction before anything is written.
KNOWN_FAILURES = {
    ("TestPostgres", "test_array_offset"),
    ("TestClickhouse", "test_array_offset"),
    ("TestDuckDB", "test_array_index"),
    ("TestBigQuery", "test_warnings"),
}


def _test_key(test):
    # test.id() is e.g. "tests.dialects.test_postgres.TestPostgres.test_array_offset";
    # the last two dotted parts are (class, method).
    parts = test.id().split(".")
    if len(parts) >= 2:
        return (parts[-2], parts[-1])
    return ("", test.id())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dialects", help="comma-separated dialects to replace; default: all six, full rewrite"
    )
    parser.add_argument(
        "--transpile-only",
        action="store_true",
        help="extract only testdata/transpile_corpus.jsonl and leave testdata/dialect_identity.jsonl "
        "byte-identical (a full run rewrites every section, dropping the hand-added MySQL/Postgres/"
        "base records the per-dialect --dialects runs preserve verbatim)",
    )
    parser.add_argument(
        "--allow-failures",
        action="store_true",
        help="record what was captured instead of aborting on unittest failures (use while "
        "bringing a new dialect up; the corpus is a burndown list, not a release gate)",
    )
    args = parser.parse_args()
    if args.transpile_only and args.dialects is not None:
        parser.error("--transpile-only always extracts every dialect; drop --dialects")
    selected = set(DIALECTS if args.dialects is None else args.dialects.split(","))
    unknown = selected.difference(DIALECTS)
    if unknown:
        parser.error("unknown dialect(s): " + ", ".join(sorted(unknown)))
    if args.dialects is None:
        # Full runs also extract the base dialect (TestDialect, dialect = None -> "").
        selected.add("")

    runner = unittest.TextTestRunner(verbosity=0)
    unexpected = []
    for cls in (TestBigQuery, TestDuckDB, TestSnowflake, TestMySQL, TestPostgres, TestAthena, TestTrino, TestPresto, TestHive, TestTSQL, TestClickhouse, TestStarrocks, TestDoris, TestOracle, TestRedshift, TestMaterialize, TestRisingWave, TestSpark, TestDatabricks, TestDialect):
        # TestDialect carries dialect = None (base); _norm_dialect maps it to "", which only a
        # full run adds to `selected` above — a partial --dialects run skips it (and skips
        # writing the transpile corpus anyway).
        if _norm_dialect(cls.dialect) not in selected:
            continue
        result = runner.run(unittest.defaultTestLoader.loadTestsFromTestCase(cls))
        for test, _ in list(result.failures) + list(result.errors):
            key = _test_key(test)
            if key not in KNOWN_FAILURES:
                unexpected.append(".".join(key))
        # With Python >= 3.11 enforced above, the sole skip guard (test_mysql.py:912)
        # no longer fires, so NO test in the selected classes should skip. A skip here means a
        # method — and its validate_identity calls — silently dropped out of the corpus,
        # so treat it as a blocker just like a failure.
        for test, _ in list(result.skipped):
            unexpected.append(".".join(_test_key(test)) + " (skipped)")

    if unexpected and not args.allow_failures:
        print(
            "Refusing to write corpus: the unexpected unittest failure(s)/skip(s) below may "
            "have truncated a test method before its validate_identity calls fired. Fix the "
            "extractor (or, once verified corpus-irrelevant, add to KNOWN_FAILURES) and "
            "re-run:\n  " + "\n  ".join(sorted(set(unexpected))),
            file=sys.stderr,
        )
        sys.exit(1)

    if args.transpile_only:
        # Do not touch the identity corpus: a full rewrite drops the hand-added MySQL/Postgres/
        # base records that partial --dialects runs preserve verbatim (see --transpile-only help).
        print(
            "transpile-only run: testdata/dialect_identity.jsonl left unchanged",
            file=sys.stderr,
        )
    else:
        records = {}
        for rec in captured:
            if rec["dialect"] not in selected:
                continue
            key = (rec["dialect"], rec["sql"], rec["want"], rec["pretty"], rec.get("identify", False))
            records[key] = rec

        deduped = sorted(
            records.values(), key=lambda r: (r["dialect"], r["sql"], r["want"])
        )

        out_path = "testdata/dialect_identity.jsonl"
        preserved = {}
        if args.dialects is not None and Path(out_path).exists():
            for line in Path(out_path).read_text().splitlines():
                if line.strip():
                    dialect = json.loads(line)["dialect"]
                    if dialect not in selected:
                        preserved.setdefault(dialect, []).append(line)
        output = {}
        for rec in deduped:
            output.setdefault(rec["dialect"], []).append(json.dumps(rec))
        output.update(preserved)
        with open(out_path, "w") as f:
            for dialect in sorted(d for d in output if d):
                for line in output[dialect]:
                    f.write(line + "\n")
            for line in output.get("", []):
                f.write(line + "\n")

        print(f"captured {len(captured)} validate_identity call(s)", file=sys.stderr)
        for dialect in sorted(output):
            print(f"  {dialect or 'base'}: {len(output[dialect])}", file=sys.stderr)
        count = sum(len(lines) for lines in output.values())
        print(f"wrote {count} deduped record(s) to {out_path}", file=sys.stderr)

    if args.dialects is not None:
        # A partial run only fires validate_all for the selected classes, so rewriting here
        # would drop every pair owned by the other classes. Transpile extraction is full-run-only.
        print(
            "partial run: testdata/transpile_corpus.jsonl left unchanged",
            file=sys.stderr,
        )
        return

    tp_records = {}
    for rec in transpile_captured:
        key = (rec["read"], rec["write"], rec["sql"], rec["want"], rec["pretty"], rec["identify"])
        tp_records[key] = rec
    tp_deduped = sorted(
        tp_records.values(),
        key=lambda r: (r["read"], r["write"], r["sql"], r["want"], r["pretty"], r["identify"]),
    )
    tp_path = "testdata/transpile_corpus.jsonl"
    with open(tp_path, "w") as f:
        for rec in tp_deduped:
            f.write(json.dumps(rec) + "\n")

    pair_counts = {}
    for rec in tp_deduped:
        pair = (rec["read"], rec["write"])
        pair_counts[pair] = pair_counts.get(pair, 0) + 1
    print(
        f"wrote {len(tp_deduped)} deduped transpile record(s) "
        f"across {len(pair_counts)} dialect pair(s) to {tp_path}",
        file=sys.stderr,
    )
    for (read_dialect, write_dialect), n in sorted(pair_counts.items()):
        print(f"  {read_dialect or 'base'} -> {write_dialect or 'base'}: {n}", file=sys.stderr)


if __name__ == "__main__":
    main()
