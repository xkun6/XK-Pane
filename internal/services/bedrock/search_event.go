package bedrock

// search_event.go：event 搜索/查询，复用 search_api.go 的 score_and_rank / lookup_filtered /
// render_* / validate_side（label 传 "event"）。对齐 search/event.rs。

// searchEvent ← bedrock_search_event。
func searchEvent(name string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	return renderSearchResults("event", name, scoreAndRank(idx.allEvents, name, 20)), nil
}

// getEvent ← bedrock_get_event。
func getEvent(name string, side *string) (string, error) {
	if side != nil {
		if err := validateSide(*side); err != nil {
			return "", err
		}
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	entries := lookupFiltered(idx.eventByName, name, side)
	if len(entries) == 0 {
		return notFoundWithSuggestions(idx, "event", name, side), nil
	}
	return renderEntriesMarkdown(entries), nil
}
