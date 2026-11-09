"""Generate optimizer/typing_metadata.go from the pinned sqlglot's EXPRESSION_METADATA.

Run from the repo root, with the pinned reference fetched:

    PYTHONPATH=.reference/sqlglot-v30.17.0 python3 -I scripts/gen_typing_metadata.py

It introspects each dialect's resolved EXPRESSION_METADATA (sqlglot/typing/*.py merged through
the dialect class hierarchy), so subclass expansion matches upstream exactly. Base entries are
emitted for the "" dialect; every registered Go dialect gets only the entries that differ from
base. A lambda annotator is identified by its source line and mapped to hand-written Go in
ANNOTATORS below; an unmapped one fails the run.
"""

import ast
import inspect
import os
import sys

sys.path.insert(0, os.environ.get("PYTHONPATH", ".reference/sqlglot-v30.17.0"))

from sqlglot import exp  # noqa: E402
from sqlglot.dialects.dialect import Dialect  # noqa: E402

GO_DIALECTS = [
    "mysql", "postgres", "presto", "trino", "hive", "athena", "tsql",
    "clickhouse", "starrocks", "doris", "oracle", "redshift", "materialize",
]

# Lambda / function source -> Go annotator expression (a func(*TypeAnnotator, exp.Expression)).
ANNOTATORS = {
    "lambda self, e: self._annotate_binary(e)": "annBinary",
    "lambda self, e: self._annotate_unary(e)": "annUnary",
    "lambda self, e: self._set_type(e, self.schema.get_udf_type(e))": "annUDF",
    "lambda self, e: self._annotate_by_args(e, 'expressions', array=True)": "annByArgs(annotateArgs{keys: []string{\"expressions\"}, array: true})",
    "lambda self, e: self._annotate_by_args(e, 'this', array=True)": "annByArgs(annotateArgs{keys: []string{\"this\"}, array: true})",
    "lambda self, e: self._annotate_bracket(e)": "annBracket",
    "lambda self, e: self._annotate_by_args(e, *[if_expr.args['true'] for if_expr in e.args['ifs']], 'default')": "annCase",
    "lambda self, e: self._set_type(e, exp.DType.BIGINT if e.args.get('big_int') else exp.DType.INT)": "annBigIntFlag(\"big_int\")",
    "lambda _, e: e": "annNoop",
    "lambda self, e: self._annotate_div(e)": "annDiv",
    "lambda self, e: self._annotate_by_args(e, 'expressions')": "annByArgs(byKeys(\"expressions\"))",
    "lambda self, e: self._annotate_dot(e)": "annDot",
    "lambda self, e: self._annotate_explode(e)": "annExplode",
    "lambda self, e: self._annotate_extract(e)": "annExtract",
    "lambda self, e: self._set_type(e, exp.DType.BIGINT if e.args.get('is_integer') else exp.DType.BINARY)": "annHexString",
    "lambda self, e: self._annotate_by_args(e, 'start', 'end', 'step', array=True)": "annByArgs(annotateArgs{keys: []string{\"start\", \"end\", \"step\"}, array: true})",
    "lambda self, e: self._set_type(e, exp.DataType.from_str('ARRAY<DATE>'))": "annBuild(\"ARRAY<DATE>\", \"\")",
    "lambda self, e: self._set_type(e, exp.DataType.from_str('ARRAY<TIMESTAMP>'))": "annBuild(\"ARRAY<TIMESTAMP>\", \"\")",
    "lambda self, e: self._annotate_by_args(e, 'true', 'false')": "annByArgs(byKeys(\"true\", \"false\"))",
    "lambda self, e: self._annotate_by_args(e, 'this', 'default')": "annByArgs(byKeys(\"this\", \"default\"))",
    "lambda self, e: self._annotate_literal(e)": "annLiteral",
    "lambda self, e: self._annotate_by_args(e, 'this', 'expression')": "annByArgs(byKeys(\"this\", \"expression\"))",
    "lambda self, e: self._annotate_by_args(e, 'expression')": "annByArgs(byKeys(\"expression\"))",
    "lambda self, e: self._annotate_struct(e)": "annStruct",
    "lambda self, e: self._annotate_by_args(e, 'this', 'expressions', promote=True)": "annByArgs(annotateArgs{keys: []string{\"this\", \"expressions\"}, promote: true})",
    "lambda self, e: self._set_type(e, exp.DType.TIMESTAMPTZ if e.args.get('with_tz') else exp.DType.TIMESTAMP)": "annTimestamp",
    "lambda self, e: self._annotate_to_map(e)": "annToMap",
    "lambda self, e: self._annotate_unnest(e)": "annUnnest",
    "lambda self, e: self._annotate_within_group(e)": "annWithinGroup",
    "lambda self, e: self._annotate_subquery(e)": "annSubquery",
    "lambda self, e: self._annotate_by_args(e, 'this')": "annByArgs(byKeys(\"this\"))",
    "lambda self, e: self._annotate_by_args(e, 'this', 'expressions')": "annByArgs(byKeys(\"this\", \"expressions\"))",
    "lambda self, e: self._annotate_by_array_element(e)": "annByArrayElement",
    "lambda self, e: self._annotate_timeunit(e)": "annTimeunit",
    "lambda self, e: self._set_type(e, e.args['to'])": "annCast",
    "lambda self, e: self._annotate_map(e)": "annMap",
    "lambda self, e: self._annotate_by_args(e, 'this') if e.this else self._set_type(e, exp.DType.DOUBLE)": "annPrestoRand",
    "lambda self, e: self._annotate_by_args(e, 'quantile')": "annByArgs(byKeys(\"quantile\"))",
    "lambda self, e: self._annotate_by_args(e, 'true', 'false', promote=True)": "annByArgs(annotateArgs{keys: []string{\"true\", \"false\"}, promote: true})",
    "lambda self, e: self._set_type(e, exp.DataType.build('FixedString(16)', dialect='clickhouse'))": "annBuild(\"FixedString(16)\", \"clickhouse\")",
    "lambda self, e: self._set_type(e, exp.DataType.build('Float64', dialect='clickhouse'))": "annBuild(\"Float64\", \"clickhouse\")",
    "_annotate_reverse": "annMySQLReverse",
    "_annotate_truncate": "annMySQLTruncate",
    "_annotate_regexp_replace": "annMySQLRegexpReplace",
    "_annotate_compress": "annMySQLCompress",
    "_annotate_bit_func": "annMySQLBitFunc",
}

_lambda_cache: dict = {}


def annotator_source(fn) -> str:
    if fn.__name__ != "<lambda>":
        return fn.__name__
    path = inspect.getsourcefile(fn)
    if path not in _lambda_cache:
        tree = ast.parse(open(path).read())
        _lambda_cache[path] = {
            node.lineno: ast.unparse(node) for node in ast.walk(tree) if isinstance(node, ast.Lambda)
        }
    return _lambda_cache[path][fn.__code__.co_firstlineno]


def go_dtype(dtype) -> str:
    return f'dtype("{dtype.value}")'


def go_datatype(dt) -> str:
    if isinstance(dt, exp.DType):
        return go_dtype(dt)
    args = [go_dtype(dt.this)]
    for e in dt.expressions:
        args.append(go_datatype(e))
    return f"dataTypeOf({', '.join(args)})"


def go_spec(spec) -> str:
    if "annotator" in spec:
        src = annotator_source(spec["annotator"])
        if src not in ANNOTATORS:
            raise SystemExit(f"unmapped annotator: {src}")
        return f"typeSpec{{annotator: {ANNOTATORS[src]}}}"
    returns = spec["returns"]
    return f"typeSpec{{returns: {go_datatype(returns)}}}"


def spec_key(spec):
    if "annotator" in spec:
        return ("a", annotator_source(spec["annotator"]))
    r = spec["returns"]
    return ("r", r.sql() if isinstance(r, exp.DataType) else r.value)


def entries(metadata: dict) -> dict:
    return {cls.__name__: spec for cls, spec in metadata.items()}


def emit(name: str, specs: dict, out: list) -> None:
    out.append(f'\tregisterTypingMetadata("{name}", map[string]typeSpec{{')
    for cls_name in sorted(specs):
        out.append(f'\t\t"{cls_name}": {go_spec(specs[cls_name])},')
    out.append("\t})")


def main() -> None:
    base = entries(Dialect().EXPRESSION_METADATA)
    out = [
        "// Code generated by scripts/gen_typing_metadata.py from sqlglot v30.17.0; DO NOT EDIT.",
        "",
        "package optimizer",
        "",
        'import exp "github.com/maltzsama/sqlglot-go/expressions"',
        "",
        "func init() {",
    ]
    emit("", base, out)
    for dialect in GO_DIALECTS:
        resolved = entries(Dialect.get_or_raise(dialect).EXPRESSION_METADATA)
        overlay = {
            name: spec
            for name, spec in resolved.items()
            if name not in base or spec_key(base[name]) != spec_key(spec)
        }
        removed = [name for name in base if name not in resolved]
        if removed:
            raise SystemExit(f"{dialect} removes base entries {removed}; not supported")
        if overlay:
            emit(dialect, overlay, out)
    out.append("}")
    out.append("")
    # exp is used by the generated annotators that name DTypes or build nodes.
    out.append("var _ = exp.DTypeUnknown")
    out.append("")
    with open("optimizer/typing_metadata.go", "w") as f:
        f.write("\n".join(out))


if __name__ == "__main__":
    main()
