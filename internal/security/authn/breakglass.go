package authn

import (
	"errors"
	"sort"
)

type BreakGlassRecord struct {
	KeyholderID string
	Salt        string
	Iterations  int
	Hash        string
}

func NewBreakGlassRecord(keyholderID, key string, iterations int) (BreakGlassRecord, error) {
	p, err := NewPasswordRecord(keyholderID, key, iterations)
	if err != nil {
		return BreakGlassRecord{}, err
	}
	return BreakGlassRecord{KeyholderID: p.HumanID, Salt: p.Salt, Iterations: p.Iterations, Hash: p.Hash}, nil
}

// VerifyBreakGlass requires exactly two distinct registered offline keyholders.
// It authenticates only. It does not grant role, incident, compartment or output authority.
func VerifyBreakGlass(records []BreakGlassRecord, presented map[string]string) ([]string, error) {
	if len(presented) != 2 {
		return nil, errors.New("exactly two break-glass keyholders required")
	}
	byID := make(map[string]BreakGlassRecord, len(records))
	for _, record := range records {
		if record.KeyholderID == "" {
			return nil, errors.New("break-glass keyholder id is required")
		}
		if _, exists := byID[record.KeyholderID]; exists {
			return nil, errors.New("duplicate break-glass keyholder")
		}
		if err := ValidatePasswordRecord(PasswordRecord{HumanID: record.KeyholderID, Salt: record.Salt, Iterations: record.Iterations, Hash: record.Hash}); err != nil {
			return nil, err
		}
		byID[record.KeyholderID] = record
	}
	ids := make([]string, 0, 2)
	for id, key := range presented {
		record, exists := byID[id]
		if !exists {
			return nil, errors.New("break-glass credentials rejected")
		}
		password := PasswordRecord{HumanID: id, Salt: record.Salt, Iterations: record.Iterations, Hash: record.Hash}
		if !VerifyPassword(password, key) {
			return nil, errors.New("break-glass credentials rejected")
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
