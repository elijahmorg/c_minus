package lsp

import (
	"encoding/json"
	"fmt"
)

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

func mapPositionCToCM(lm *lineMapper, pos lspPosition) (string, lspPosition, error) {
	origFile, origLine1 := lm.mapLine(pos.Line + 1)
	if origFile == "" {
		return "", lspPosition{}, fmt.Errorf("no line mapping")
	}
	return origFile, lspPosition{Line: origLine1 - 1, Character: pos.Character}, nil
}

func mapRangeCToCM(lm *lineMapper, r lspRange) (string, lspRange, error) {
	file1, start, err := mapPositionCToCM(lm, r.Start)
	if err != nil {
		return "", lspRange{}, err
	}
	file2, end, err := mapPositionCToCM(lm, r.End)
	if err != nil {
		return "", lspRange{}, err
	}
	if file1 != file2 {
		// clangd can theoretically return a range crossing files; ignore mapping in that case.
		return "", lspRange{}, fmt.Errorf("range crosses files")
	}
	return file1, lspRange{Start: start, End: end}, nil
}

func mapHoverResultToCM(lm *lineMapper, raw json.RawMessage) (json.RawMessage, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, "", nil
	}

	var h map[string]any
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, "", err
	}

	// Range is optional.
	rawRange, ok := h["range"]
	if !ok {
		b, _ := json.Marshal(h)
		return b, "", nil
	}

	b, err := json.Marshal(rawRange)
	if err != nil {
		return nil, "", err
	}
	var rr lspRange
	if err := json.Unmarshal(b, &rr); err != nil {
		return nil, "", err
	}

	file, mapped, err := mapRangeCToCM(lm, rr)
	if err != nil {
		b, _ := json.Marshal(h)
		return b, "", nil
	}

	h["range"] = mapped
	out, _ := json.Marshal(h)
	return out, file, nil
}

func mapDefinitionResultToCM(lm *lineMapper, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}

	// Could be a Location, []Location, or []LocationLink.
	var anyVal any
	if err := json.Unmarshal(raw, &anyVal); err != nil {
		return nil, err
	}

	mapped := mapLocationsAny(lm, anyVal)
	out, err := json.Marshal(mapped)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func mapLocationsAny(lm *lineMapper, v any) any {
	switch vv := v.(type) {
	case []any:
		out := make([]any, 0, len(vv))
		for _, e := range vv {
			out = append(out, mapLocationsAny(lm, e))
		}
		return out
	case map[string]any:
		// Location: {uri, range}
		if _, ok := vv["uri"]; ok {
			if r, ok := vv["range"]; ok {
				b, _ := json.Marshal(r)
				var rr lspRange
				if json.Unmarshal(b, &rr) == nil {
					file, mapped, err := mapRangeCToCM(lm, rr)
					if err == nil {
						if cmURI, uerr := fileURIFromPath(file); uerr == nil {
							vv["uri"] = cmURI
							vv["range"] = mapped
						}
					}
				}
			}
			return vv
		}

		// LocationLink: {targetUri, targetRange, targetSelectionRange}
		if _, ok := vv["targetUri"]; ok {
			vv = mapLocationLink(lm, vv)
			return vv
		}

		// Unknown object shape
		return vv
	default:
		return v
	}
}

func mapLocationLink(lm *lineMapper, ll map[string]any) map[string]any {
	// Map the target range if possible.
	if tr, ok := ll["targetRange"]; ok {
		b, _ := json.Marshal(tr)
		var rr lspRange
		if json.Unmarshal(b, &rr) == nil {
			file, mapped, err := mapRangeCToCM(lm, rr)
			if err == nil {
				if cmURI, uerr := fileURIFromPath(file); uerr == nil {
					ll["targetUri"] = cmURI
					ll["targetRange"] = mapped
				}
			}
		}
	}

	if tsr, ok := ll["targetSelectionRange"]; ok {
		b, _ := json.Marshal(tsr)
		var rr lspRange
		if json.Unmarshal(b, &rr) == nil {
			_, mapped, err := mapRangeCToCM(lm, rr)
			if err == nil {
				ll["targetSelectionRange"] = mapped
			}
		}
	}

	return ll
}
