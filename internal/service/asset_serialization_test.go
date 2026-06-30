package service

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// mblAssetStorageValue mirrors the interpreter's unexported mblToStorage():
// it serializes an MBL value exactly as deploy_pwa() does before staging the
// write. Kept here so the test exercises the real production byte format.
func mblAssetStorageValue(value interface{}) (storage.Value, error) {
	tv, err := types.CreateValue(value)
	if err != nil {
		return storage.Value{}, err
	}
	return storage.Value{TypeTag: tv.TypeTag(), Data: tv.Serialize()}, nil
}

// TestParseAssetFromRealSerializedRecord is the integration guard for the
// deploy → serve seam. deploy_pwa() builds a types.Record{data, mime_type} and
// stores it as storage.Value{TypeTag: TypeRecord, Data: record.Serialize()}.
// The asset cache must be able to read that *exact* byte format back.
//
// Before the fix, parseAssetFromStorageValue only understood a fictional
// `{"data":"...","mime_type":"..."}` JSON string used by the test mocks — it
// could not parse the real binary record format, so every asset published by
// the production deploy_pwa() path was silently dropped at serving time.
func TestParseAssetFromRealSerializedRecord(t *testing.T) {
	acm := &AssetCacheManager{}

	const (
		wantData = "<!DOCTYPE html><html><body>real</body></html>"
		wantMime = "text/html; charset=utf-8"
	)

	// Reproduce exactly what the interpreter's mblToStorage() does for an
	// asset record: types.CreateValue(record).Serialize().
	record := types.Record{
		Fields: map[string]interface{}{
			"data":      types.Text{Value: wantData},
			"mime_type": types.Text{Value: wantMime},
		},
	}
	storageValue, err := mblAssetStorageValue(record)
	if err != nil {
		t.Fatalf("serializing asset record: %v", err)
	}

	asset := acm.parseAssetFromStorageValue(storageValue, "index.html")
	if asset == nil {
		t.Fatal("parseAssetFromStorageValue returned nil for a real serialized record")
	}
	if string(asset.Data) != wantData {
		t.Errorf("asset data = %q, want %q", string(asset.Data), wantData)
	}
	if asset.MimeType != wantMime {
		t.Errorf("asset mime = %q, want %q", asset.MimeType, wantMime)
	}
	if asset.Path != "index.html" {
		t.Errorf("asset path = %q, want index.html", asset.Path)
	}
}
