package yelp

import (
	"context"

	"github.com/tamnd/any-cli/kit/errs"
)

// ops.go holds the handler for every operation declared in domain.go. kit
// reflects each input struct into CLI flags, HTTP query params, and MCP tool
// arguments: kit:"arg" is a positional, kit:"flag,inherit" binds the shared
// --limit, and kit:"inject" receives the client newClient builds. The reference
// ops (id, url) take no client; they run offline.

// --- search ---

type searchIn struct {
	Term     string  `kit:"arg" help:"what to search for (a term, a category, a business name)"`
	Location string  `kit:"arg,optional" help:"a place to search in, e.g. \"Oakland, CA\" (or use --location)"`
	Limit    int     `kit:"flag,inherit"`
	Client   *Client `kit:"inject"`
}

func search(ctx context.Context, in searchIn, emit func(*Business) error) error {
	items, err := in.Client.Search(ctx, in.Term, in.Location, limitOr(in.Limit, defaultLimit))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

// --- business ---

type bizRef struct {
	Alias  string  `kit:"arg" help:"business alias or /biz/ URL"`
	Client *Client `kit:"inject"`
}

func getBiz(ctx context.Context, in bizRef, emit func(*Business) error) error {
	b, err := in.Client.Business(ctx, in.Alias)
	if err != nil {
		return mapErr(err)
	}
	return emit(b)
}

type bizListIn struct {
	Alias  string  `kit:"arg" help:"business alias or /biz/ URL"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func reviews(ctx context.Context, in bizListIn, emit func(*Review) error) error {
	rs, err := in.Client.Reviews(ctx, in.Alias, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(rs, emit)
}

// --- user ---

type userRef struct {
	ID     string  `kit:"arg" help:"user id or /user_details URL"`
	Client *Client `kit:"inject"`
}

func getUser(ctx context.Context, in userRef, emit func(*User) error) error {
	u, err := in.Client.User(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(u)
}

// --- suggest ---

type prefixIn struct {
	Prefix string  `kit:"arg" help:"the typed prefix"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func suggest(ctx context.Context, in prefixIn, emit func(*Suggestion) error) error {
	ss, err := in.Client.Suggest(ctx, in.Prefix, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(ss, emit)
}

// --- categories ---

type categoriesIn struct {
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func categories(ctx context.Context, in categoriesIn, emit func(*Category) error) error {
	cs, err := in.Client.Categories(ctx, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(cs, emit)
}

// --- category (one alias) ---

type categoryRefIn struct {
	Alias  string  `kit:"arg" help:"category alias, e.g. \"coffee\""`
	Client *Client `kit:"inject"`
}

func getCategory(ctx context.Context, in categoryRefIn, emit func(*Category) error) error {
	cat, err := in.Client.Category(ctx, in.Alias)
	if err != nil {
		return mapErr(err)
	}
	return emit(cat)
}

// --- reference tools (offline) ---

type refIn struct {
	Ref string `kit:"arg" help:"any Yelp URL, path, alias, or id"`
}

func classifyRef(_ context.Context, in refIn, emit func(*Ref) error) error {
	r := Classify(in.Ref)
	if r.Kind == "unknown" {
		return errs.Usage("unrecognized yelp reference: %q", in.Ref)
	}
	return emit(&r)
}

type urlIn struct {
	Kind string `kit:"arg" help:"biz, user, or category"`
	ID   string `kit:"arg" help:"the id for that kind (a business alias, a user id, or a category alias)"`
}

func buildURL(_ context.Context, in urlIn, emit func(*Ref) error) error {
	u := URLFor(in.Kind, in.ID)
	if u == "" {
		return errs.Usage("yelp has no resource type %q", in.Kind)
	}
	return emit(&Ref{Input: in.Kind + "/" + in.ID, Kind: in.Kind, ID: in.ID, URL: u})
}

// emitAll streams a slice of records through emit.
func emitAll[T any](items []*T, emit func(*T) error) error {
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}

// limitOr returns the operator's --limit when set, else the command's own
// default fetch count.
func limitOr(limit, def int) int {
	if limit > 0 {
		return limit
	}
	return def
}
