package optimizer

import (
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/schema"
)

// RuleArgs carries the keyword arguments optimize offers its rules: upstream's possible_kwargs
// (optimizer/optimizer.py:93-102). Upstream inspects each rule's signature and passes only the
// parameters it declares; in Go every rule receives the whole set and reads the fields it
// takes. A nil pointer field means "not passed", so the rule keeps its own default.
type RuleArgs struct {
	// Schema is the column-metadata schema, already ensured.
	Schema schema.Schema
	// DefaultSchema is upstream's db: the schema-level qualifier for unqualified tables (named
	// as in QualifyOpts, see DEVIATIONS.md §7).
	DefaultSchema exp.IdentifierName
	Catalog       exp.IdentifierName
	Dialect       dialects.DialectType
	SQL           string
	// IsolateTables and QuoteIdentifiers are the values optimize hands to qualify: true, as
	// other optimizations need it to perform well, and false, as quote_identifiers runs later.
	IsolateTables    bool
	QuoteIdentifiers bool

	// The rest are optimize's **kwargs, forwarded to the rules that take them.

	// qualify
	ExpandAliasRefs           *bool
	ExpandStars               *bool
	InferSchema               *bool
	QualifyColumns            *bool
	AllowPartialQualification *bool
	ValidateQualifyColumns    *bool
	CanonicalizeTableAliases  *bool
	OnQualify                 func(exp.Expression)
	// Identify is read by qualify and quote_identifiers.
	Identify *bool
	// pushdown_projections
	RemoveUnusedSelections *bool
	// Journal is read by pushdown_projections and eliminate_ctes.
	Journal *Journal
	// normalize
	DNF         *bool
	MaxDistance *int
	// merge_subqueries
	LeaveTablesIsolated *bool
	// simplify
	ConstantPropagation    *bool
	CoalesceSimplification *bool
}

// Rule is an optimizer rule (upstream's OptimizerFn, optimizer/optimizer.py:25-37): it takes an
// expression and the optimize arguments, and returns the rewritten expression.
type Rule func(expression exp.Expression, args *RuleArgs) exp.Expression

func boolOr(v *bool, def bool) bool {
	if v != nil {
		return *v
	}
	return def
}

// QualifyRule runs Qualify with the optimize arguments.
func QualifyRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	opts := DefaultQualifyOpts()
	opts.Dialect = args.Dialect
	opts.DefaultSchema = args.DefaultSchema
	opts.Catalog = args.Catalog
	opts.Schema = args.Schema
	opts.SQL = args.SQL
	opts.IsolateTables = args.IsolateTables
	opts.QuoteIdentifiers = args.QuoteIdentifiers
	opts.ExpandAliasRefs = boolOr(args.ExpandAliasRefs, opts.ExpandAliasRefs)
	opts.ExpandStars = boolOr(args.ExpandStars, opts.ExpandStars)
	if args.InferSchema != nil {
		opts.InferSchema = args.InferSchema
	}
	opts.QualifyColumns = boolOr(args.QualifyColumns, opts.QualifyColumns)
	opts.AllowPartialQualification = boolOr(args.AllowPartialQualification, opts.AllowPartialQualification)
	opts.ValidateQualifyColumns = boolOr(args.ValidateQualifyColumns, opts.ValidateQualifyColumns)
	opts.Identify = boolOr(args.Identify, opts.Identify)
	opts.CanonicalizeTableAliases = boolOr(args.CanonicalizeTableAliases, opts.CanonicalizeTableAliases)
	if args.OnQualify != nil {
		opts.OnQualify = args.OnQualify
	}
	return Qualify(expression, opts)
}

// PushdownProjectionsRule runs PushdownProjections with the optimize arguments.
func PushdownProjectionsRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return PushdownProjections(expression, args.Schema, boolOr(args.RemoveUnusedSelections, true), args.Dialect, args.Journal)
}

// NormalizeRule runs Normalize with the optimize arguments.
func NormalizeRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	maxDistance := 128
	if args.MaxDistance != nil {
		maxDistance = *args.MaxDistance
	}
	return Normalize(expression, boolOr(args.DNF, false), maxDistance)
}

// UnnestSubqueriesRule runs UnnestSubqueries.
func UnnestSubqueriesRule(expression exp.Expression, _ *RuleArgs) exp.Expression {
	return UnnestSubqueries(expression)
}

// PushdownPredicatesRule runs PushdownPredicates with the optimize arguments.
func PushdownPredicatesRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return PushdownPredicates(expression, args.Dialect)
}

// OptimizeJoinsRule runs OptimizeJoins.
func OptimizeJoinsRule(expression exp.Expression, _ *RuleArgs) exp.Expression {
	return OptimizeJoins(expression)
}

// EliminateSubqueriesRule runs EliminateSubqueries.
func EliminateSubqueriesRule(expression exp.Expression, _ *RuleArgs) exp.Expression {
	return EliminateSubqueries(expression)
}

// MergeSubqueriesRule runs MergeSubqueries with the optimize arguments.
func MergeSubqueriesRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return MergeSubqueries(expression, boolOr(args.LeaveTablesIsolated, false))
}

// EliminateJoinsRule runs EliminateJoins.
func EliminateJoinsRule(expression exp.Expression, _ *RuleArgs) exp.Expression {
	return EliminateJoins(expression)
}

// EliminateCTEsRule runs EliminateCTEs with the optimize arguments.
func EliminateCTEsRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return EliminateCTEs(expression, args.Journal)
}

// QuoteIdentifiersRule runs QuoteIdentifiers with the optimize arguments.
func QuoteIdentifiersRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return QuoteIdentifiers(expression, args.Dialect, boolOr(args.Identify, true))
}

// AnnotateTypesRule runs AnnotateTypes with the optimize arguments. The Go AnnotateTypes takes
// no expression_metadata, coerces_to or overwrite_types, so those keep their defaults.
func AnnotateTypesRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return AnnotateTypes(expression, args.Schema, args.Dialect)
}

// CanonicalizeRule runs Canonicalize with the optimize arguments.
func CanonicalizeRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return Canonicalize(expression, args.Dialect)
}

// SimplifyRule runs Simplify with the optimize arguments.
func SimplifyRule(expression exp.Expression, args *RuleArgs) exp.Expression {
	return Simplify(expression, boolOr(args.ConstantPropagation, false), boolOr(args.CoalesceSimplification, false), args.Dialect)
}

// Rules ports RULES (optimizer/optimizer.py:40-55): the default rule sequence, in order.
var Rules = []Rule{
	QualifyRule,
	PushdownProjectionsRule,
	NormalizeRule,
	UnnestSubqueriesRule,
	PushdownPredicatesRule,
	OptimizeJoinsRule,
	EliminateSubqueriesRule,
	MergeSubqueriesRule,
	EliminateJoinsRule,
	EliminateCTEsRule,
	QuoteIdentifiersRule,
	AnnotateTypesRule,
	CanonicalizeRule,
	SimplifyRule,
}

// OptimizeOpts holds optimize's arguments. Its zero value runs the default rules with no
// schema under the base dialect.
type OptimizeOpts struct {
	// Schema is the column-metadata schema; see schema.SchemaType for the accepted forms.
	Schema schema.SchemaType
	// DefaultSchema is upstream's db: the default schema, as a USE statement might set it.
	DefaultSchema exp.IdentifierName
	// Catalog is the default catalog, as a USE CATALOG statement might set it.
	Catalog exp.IdentifierName
	Dialect dialects.DialectType
	// Rules is the rule sequence; nil means Rules. Many rules need qualified tables and
	// columns, so don't drop QualifyRule unless you know what you're doing.
	Rules []Rule
	// SQL is the original SQL string, for error highlighting.
	SQL string
	// Kwargs are the extra rule arguments (upstream's **kwargs). Its Schema, DefaultSchema,
	// Catalog, Dialect, SQL, IsolateTables and QuoteIdentifiers are set by Optimize.
	Kwargs RuleArgs
}

// Optimize ports optimize (optimizer/optimizer.py:58-114): it rewrites an AST into an optimized
// form by running each rule in turn on a copy of expression.
//
//	SELECT a FROM (SELECT x.a FROM x) AS y WHERE 1 + 1 = 2
//	  ->  SELECT "x"."a" AS "a" FROM "x" AS "x"
func Optimize(expression exp.Expression, opts OptimizeOpts) exp.Expression {
	s, err := schema.EnsureSchema(opts.Schema, opts.Dialect, true)
	if err != nil {
		panic(err)
	}
	args := opts.Kwargs
	args.Schema = s
	args.DefaultSchema = opts.DefaultSchema
	args.Catalog = opts.Catalog
	args.Dialect = opts.Dialect
	args.SQL = opts.SQL
	args.IsolateTables = true
	args.QuoteIdentifiers = false

	rules := opts.Rules
	if rules == nil {
		rules = Rules
	}
	optimized := expression.Copy()
	for _, rule := range rules {
		optimized = rule(optimized, &args)
	}
	return optimized
}
