package integration_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type sourceItem struct {
	ID                string  `json:"id"`
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	GroupID           *string `json:"group_id"`
	GroupName         *string `json:"group_name"`
	IsDefault         bool    `json:"is_default"`
	IsActive          bool    `json:"is_active"`
	RegistrationBonus *int    `json:"registration_bonus"`
	URL               string  `json:"url"`
}

func TestSourcesCRUD(t *testing.T) {
	pool := setup(t)
	data, apiSrv, redirSrv := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1/cabinet"

	// Initially: one default source created by Join.
	resp, body := doJSON(t, data.Partner, "GET", base+"/offers/"+data.Offer+"/sources", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list sources: %d %s", resp.StatusCode, body)
	}
	var list struct {
		Items []sourceItem `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Items) != 1 || !list.Items[0].IsDefault {
		t.Fatalf("expected 1 default source, got %+v", list.Items)
	}
	defID, defName, defCode := list.Items[0].ID, list.Items[0].Name, list.Items[0].Code

	// Create a group.
	resp, body = doJSON(t, data.Partner, "POST", base+"/source-groups", map[string]any{
		"name": "Telegram", "comment": "основной канал",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: %d %s", resp.StatusCode, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &group)

	// Create a source with a custom code, bound to the group.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Telegram #1", "code": "tg1", "group_id": group.ID,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create source: %d %s", resp.StatusCode, body)
	}
	var tg1 sourceItem
	_ = json.Unmarshal(body, &tg1)
	if tg1.Code != "TG1" {
		t.Fatalf("code should be normalized to TG1, got %q", tg1.Code)
	}
	if tg1.GroupID == nil || *tg1.GroupID != group.ID {
		t.Fatalf("group_id not set: %+v", tg1)
	}
	if !strings.HasSuffix(tg1.URL, "/c/TG1") {
		t.Fatalf("unexpected url: %s", tg1.URL)
	}

	// Duplicate code -> 409.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Telegram #2", "code": "TG1",
	}, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "code_taken") {
		t.Fatalf("duplicate code: want 409 code_taken, got %d %s", resp.StatusCode, body)
	}

	// Custom code resolves through the redirect and records a click.
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(redirSrv.URL + "/c/TG1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("custom code redirect: want 302, got %d", resp.StatusCode)
	}

	// Rename works.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "group_id": group.ID, "is_active": true, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update source: %d %s", resp.StatusCode, body)
	}
	var updated sourceItem
	_ = json.Unmarshal(body, &updated)
	if updated.Name != "Telegram renamed" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	// Deleting a source that already has a click -> 409 has_clicks.
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "has_clicks") {
		t.Fatalf("delete clicked source: want 409 has_clicks, got %d %s", resp.StatusCode, body)
	}

	// A clickless source can be deleted.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Temporary", "code": "TEMP",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create temp source: %d %s", resp.StatusCode, body)
	}
	var temp sourceItem
	_ = json.Unmarshal(body, &temp)
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/offers/"+data.Offer+"/sources/"+temp.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete temp source: %d %s", resp.StatusCode, body)
	}

	// Group with sources cannot be deleted.
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/source-groups/"+group.ID, nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "group_not_empty") {
		t.Fatalf("delete non-empty group: want 409 group_not_empty, got %d %s", resp.StatusCode, body)
	}

	// Unbind the source from the group, then the group deletes cleanly.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "group_id": nil, "is_active": true, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unbind source: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/source-groups/"+group.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete group: %d %s", resp.StatusCode, body)
	}

	// The last active source cannot be deactivated.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "is_active": false, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate non-last source: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+defID, map[string]any{
		"name": defName, "code": defCode, "is_active": false, "is_default": true,
	}, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "last_source") {
		t.Fatalf("deactivate last source: want 409 last_source, got %d %s", resp.StatusCode, body)
	}
}

// TestAllSourcesList checks GET /cabinet/sources: every link of the partner
// across offers in one response, with per-source totals and the offer id.
func TestAllSourcesList(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1/cabinet"

	// Create a second source on the joined offer.
	resp, body := doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Extra", "code": "EXTRA",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create source: %d %s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, data.Partner, "GET", base+"/sources", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list all sources: %d %s", resp.StatusCode, body)
	}
	var list struct {
		Items []struct {
			ID        string  `json:"id"`
			Code      string  `json:"code"`
			Name      string  `json:"name"`
			OfferID   *string `json:"offer_id"`
			URL       string  `json:"url"`
			Totals    *struct {
				Clicks int64 `json:"clicks"`
			} `json:"totals"`
			Totals30d *struct {
				Clicks int64 `json:"clicks"`
			} `json:"totals_30d"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 sources, got %+v", list.Items)
	}
	for _, it := range list.Items {
		if it.OfferID == nil || *it.OfferID != data.Offer {
			t.Fatalf("expected offer_id %s, got %+v", data.Offer, it)
		}
		if it.Totals == nil || it.Totals30d == nil {
			t.Fatalf("totals missing: %+v", it)
		}
		if it.URL == "" {
			t.Fatalf("url missing: %+v", it)
		}
	}
}

func TestLinkSourceCustomRegistrationBonus(t *testing.T) {
	pool := setup(t)
	data, apiSrv, redirSrv := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1"
	cab := base + "/cabinet"

	// 1. Negative bonus on creation is rejected
	resp, body := doJSON(t, data.Partner, "POST", cab+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Invalid Bonus Link", "code": "NEGBON", "type": "link", "registration_bonus": -500,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create source negative bonus: want 400, got %d %s", resp.StatusCode, body)
	}

	// 2. Create link source with custom registration bonus 1500
	resp, body = doJSON(t, data.Partner, "POST", cab+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Bonus Link", "code": "BONUS15", "type": "link", "registration_bonus": 1500,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create source with bonus: want 201, got %d %s", resp.StatusCode, body)
	}
	var src sourceItem
	_ = json.Unmarshal(body, &src)
	if src.RegistrationBonus == nil || *src.RegistrationBonus != 1500 {
		t.Fatalf("expected registration_bonus 1500, got %+v", src.RegistrationBonus)
	}

	// 3. Negative bonus on update is rejected
	resp, body = doJSON(t, data.Partner, "PATCH", cab+"/offers/"+data.Offer+"/sources/"+src.ID, map[string]any{
		"name": "Bonus Link", "registration_bonus": -100,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("update source negative bonus: want 400, got %d %s", resp.StatusCode, body)
	}

	// 4. Update registration bonus to 2500
	resp, body = doJSON(t, data.Partner, "PATCH", cab+"/offers/"+data.Offer+"/sources/"+src.ID, map[string]any{
		"name": "Bonus Link Updated", "registration_bonus": 2500,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update source bonus to 2500: want 200, got %d %s", resp.StatusCode, body)
	}
	_ = json.Unmarshal(body, &src)
	if src.RegistrationBonus == nil || *src.RegistrationBonus != 2500 {
		t.Fatalf("expected registration_bonus 2500, got %+v", src.RegistrationBonus)
	}

	// 5. Redirect through /c/BONUS15: verify 302 and query params
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(redirSrv.URL + "/c/BONUS15")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("redirect: want 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "ref=BONUS15") || !strings.Contains(loc, "click_token=") {
		t.Fatalf("expected Location to contain ref=BONUS15 and click_token=, got %q", loc)
	}
	parsedLoc, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse location url: %v", err)
	}
	clickToken := parsedLoc.Query().Get("click_token")
	if clickToken == "" {
		t.Fatalf("click_token empty in redirect location: %q", loc)
	}
	if parsedLoc.Query().Get("ref") != "BONUS15" {
		t.Fatalf("ref mismatch in redirect location: want BONUS15, got %q", parsedLoc.Query().Get("ref"))
	}

	// 6. Integrations lookup by click_token: resolves source and bonus 2500
	type integSource struct {
		Code              string `json:"code"`
		Type              string `json:"type"`
		IsPromo           bool   `json:"is_promo"`
		IsActive          bool   `json:"is_active"`
		AccessActive      bool   `json:"access_active"`
		RegistrationBonus *int32 `json:"registration_bonus"`
	}
	resp, body = doSignedGet(t, base+"/integrations/source?click_token="+url.QueryEscape(clickToken), data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup by click_token: want 200, got %d %s", resp.StatusCode, body)
	}
	var integ integSource
	if err := json.Unmarshal(body, &integ); err != nil {
		t.Fatalf("unmarshal integ response: %v", err)
	}
	if integ.Code != "BONUS15" || integ.RegistrationBonus == nil || *integ.RegistrationBonus != 2500 {
		t.Fatalf("unexpected lookup by click_token result: %+v", integ)
	}

	// 7. Integrations lookup with both click_token and code
	resp, body = doSignedGet(t, base+"/integrations/source?click_token="+url.QueryEscape(clickToken)+"&code=BONUS15", data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup with click_token and code: want 200, got %d %s", resp.StatusCode, body)
	}
	integ = integSource{}
	_ = json.Unmarshal(body, &integ)
	if integ.Code != "BONUS15" || integ.RegistrationBonus == nil || *integ.RegistrationBonus != 2500 {
		t.Fatalf("unexpected lookup with both result: %+v", integ)
	}

	// 8. Update source to clear registration_bonus to null (revert to casino default)
	resp, body = doJSON(t, data.Partner, "PATCH", cab+"/offers/"+data.Offer+"/sources/"+src.ID, map[string]any{
		"name": "Bonus Link Reverted", "registration_bonus": nil,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear source bonus to null: want 200, got %d %s", resp.StatusCode, body)
	}
	src = sourceItem{}
	_ = json.Unmarshal(body, &src)
	if src.RegistrationBonus != nil {
		t.Fatalf("expected registration_bonus nil after clear, got %+v", src.RegistrationBonus)
	}

	// 9. Integrations lookup by click_token after clearing bonus -> registration_bonus is omitted/nil
	resp, body = doSignedGet(t, base+"/integrations/source?click_token="+url.QueryEscape(clickToken), data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup by click_token after clear: want 200, got %d %s", resp.StatusCode, body)
	}
	integ = integSource{}
	_ = json.Unmarshal(body, &integ)
	if integ.Code != "BONUS15" || integ.RegistrationBonus != nil {
		t.Fatalf("expected nil registration_bonus after clear, got %+v", integ.RegistrationBonus)
	}

	// 10. Integrations lookup with invalid click_token and valid code -> falls back to code
	resp, body = doSignedGet(t, base+"/integrations/source?click_token=invalid.token&code=BONUS15", data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup with invalid click_token and valid code: want 200, got %d %s", resp.StatusCode, body)
	}

	// 11. Integrations lookup with invalid click_token and NO code -> 404
	resp, body = doSignedGet(t, base+"/integrations/source?click_token=invalid.token", data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("lookup with invalid click_token and no code: want 404, got %d %s", resp.StatusCode, body)
	}
}

