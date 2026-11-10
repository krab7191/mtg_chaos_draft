package db_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

var nextDraftPackID int

func seedDraftPack(t *testing.T, name string, qty, cardsPerPack int) *db.CollectionPack {
	t.Helper()
	price := 7.0
	// Each pack needs its own MTGStocks ID; AddPack merges stock for a repeated ID.
	nextDraftPackID++
	p, err := db.AddPack(context.Background(), testhelper.Pool(t), 70000+nextDraftPackID, name, name, "", "Booster", qty, 1.0, &price, cardsPerPack)
	if err != nil {
		t.Fatalf("seed pack: %v", err)
	}
	return p
}

func seedAdmin(t *testing.T) *db.User {
	t.Helper()
	u, err := db.GetOrCreateUser(context.Background(), testhelper.Pool(t), "g-drafts-admin", "a@example.com", "A", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func picksOf(packs ...*db.CollectionPack) []db.DraftPick {
	out := make([]db.DraftPick, len(packs))
	for i, p := range packs {
		id := p.ID
		out[i] = db.DraftPick{PackID: &id, SetName: p.SetName, ProductType: p.ProductType, MarketPrice: p.MarketPrice}
	}
	return out
}

func cleanDraftTables(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "drafts", "collection_packs", "users")
}

func TestListDrafts_IncludesPodSizeAndPicks(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	a := seedDraftPack(t, "Alpha", 10, 15)
	b := seedDraftPack(t, "Beta", 10, 15)

	first, err := db.CreateDraft(ctx, pool, 4, picksOf(a, b))
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateDraft(ctx, pool, 8, picksOf(b))
	if err != nil {
		t.Fatal(err)
	}

	drafts, err := db.ListDrafts(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int]db.Draft{}
	for _, d := range drafts {
		byID[d.ID] = d
	}
	if d := byID[first.ID]; d.PodSize != 4 || len(d.Picks) != 2 {
		t.Errorf("first draft: want pod 4 / 2 picks, got %d / %d", d.PodSize, len(d.Picks))
	}
	if d := byID[second.ID]; d.PodSize != 8 || len(d.Picks) != 1 || d.Picks[0].SetName != "Beta" {
		t.Errorf("second draft: unexpected %+v", d)
	}
}

func TestApproveDraft_DecrementsBySlotSize(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	normal := seedDraftPack(t, "Normal", 5, 15)    // 1 pack per slot
	epilogue := seedDraftPack(t, "Epilogue", 7, 5) // 3 packs per slot
	old := seedDraftPack(t, "Old", 4, 8)           // 2 packs per slot

	draft, err := db.CreateDraft(ctx, pool, 4, picksOf(normal, normal, epilogue, epilogue, old))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, seedAdmin(t).ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	got, err := db.GetPacksByIDs(ctx, pool, []int{normal.ID, epilogue.ID, old.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]int{normal.ID: 3, epilogue.ID: 1, old.ID: 2}
	for _, p := range got {
		if p.Quantity != want[p.ID] {
			t.Errorf("%s: want qty %d, got %d", p.Name, want[p.ID], p.Quantity)
		}
	}
}

func TestApproveDraft_InsufficientStock(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	epilogue := seedDraftPack(t, "Epilogue", 5, 5) // only 1 full slot

	draft, err := db.CreateDraft(ctx, pool, 4, picksOf(epilogue, epilogue))
	if err != nil {
		t.Fatal(err)
	}
	err = db.ApproveDraft(ctx, pool, draft.ID, seedAdmin(t).ID)
	var insuff *db.InsufficientQuantityError
	if !errors.As(err, &insuff) {
		t.Fatalf("want InsufficientQuantityError, got %v", err)
	}
	if !strings.Contains(insuff.Error(), "Epilogue") {
		t.Errorf("error should name the pack: %q", insuff.Error())
	}

	// Rolled back: stock untouched and draft still pending.
	got, _ := db.GetPacksByIDs(ctx, pool, []int{epilogue.ID})
	if got[0].Quantity != 5 {
		t.Errorf("stock should be unchanged, got %d", got[0].Quantity)
	}
	drafts, _ := db.ListDrafts(ctx, pool)
	if drafts[0].ApprovedAt != nil {
		t.Error("draft should not be approved")
	}
}

func TestApproveDraft_NotFoundOrAlreadyApproved(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	admin := seedAdmin(t)

	if err := db.ApproveDraft(ctx, pool, 999999, admin.ID); !errors.Is(err, db.ErrDraftNotFound) {
		t.Errorf("missing draft: want ErrDraftNotFound, got %v", err)
	}

	p := seedDraftPack(t, "Once", 5, 15)
	draft, err := db.CreateDraft(ctx, pool, 4, picksOf(p))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, admin.ID); !errors.Is(err, db.ErrDraftNotFound) {
		t.Errorf("second approve: want ErrDraftNotFound, got %v", err)
	}
}
