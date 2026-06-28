package tencentcloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/libdns/libdns"
)

type tencentCloudAPICall struct {
	action  string
	payload map[string]any
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withTencentCloudAPIFake(t *testing.T, respond func(action string, payload map[string]any) string) *[]tencentCloudAPICall {
	t.Helper()

	var calls []tencentCloudAPICall
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}

		action := req.Header.Get("X-TC-Action")
		calls = append(calls, tencentCloudAPICall{action: action, payload: payload})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(respond(action, payload))),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = oldClient })

	return &calls
}

func testAddressRecord() libdns.Record {
	return libdns.Address{Name: "www", IP: netip.MustParseAddr("203.0.113.10")}
}

func recordListResponse(id uint64) string {
	return `{"Response":{"RecordList":[{"RecordId":` + strconv.FormatUint(id, 10) + `}]}}`
}

func emptyRecordListResponse() string {
	return `{"Response":{"RecordList":[]}}`
}

func successResponse() string {
	return `{"Response":{"RecordId":9876}}`
}

func TestSetRecords_UsesExistingRecordIDWhenModifying(t *testing.T) {
	calls := withTencentCloudAPIFake(t, func(action string, payload map[string]any) string {
		switch action {
		case DescribeRecordList:
			return recordListResponse(1234)
		case ModifyRecord:
			return successResponse()
		default:
			t.Fatalf("unexpected Tencent Cloud action %s", action)
			return ""
		}
	})

	provider := &Provider{SecretId: "id", SecretKey: "key"}
	_, err := provider.SetRecords(context.Background(), "example.com.", []libdns.Record{testAddressRecord()})
	if err != nil {
		t.Fatalf("SetRecords returned error: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("SetRecords should find then modify an existing record, got calls=%+v", *calls)
	}
	if (*calls)[1].action != ModifyRecord {
		t.Fatalf("SetRecords should modify an existing record, got %s", (*calls)[1].action)
	}
	id, ok := (*calls)[1].payload["RecordId"].(float64)
	if !ok || uint64(id) != 1234 {
		t.Fatalf("ModifyRecord must use the RecordId returned by DescribeRecordList, got payload=%+v", (*calls)[1].payload)
	}
}

func TestDeleteRecords_DeletesExistingRecordID(t *testing.T) {
	calls := withTencentCloudAPIFake(t, func(action string, payload map[string]any) string {
		switch action {
		case DescribeRecordList:
			return recordListResponse(4321)
		case DeleteRecord:
			return successResponse()
		default:
			t.Fatalf("unexpected Tencent Cloud action %s", action)
			return ""
		}
	})

	provider := &Provider{SecretId: "id", SecretKey: "key"}
	_, err := provider.DeleteRecords(context.Background(), "example.com.", []libdns.Record{testAddressRecord()})
	if err != nil {
		t.Fatalf("DeleteRecords returned error: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("DeleteRecords should find then delete an existing record, got calls=%+v", *calls)
	}
	if (*calls)[1].action != DeleteRecord {
		t.Fatalf("DeleteRecords should delete an existing record, got %s", (*calls)[1].action)
	}
	id, ok := (*calls)[1].payload["RecordId"].(float64)
	if !ok || uint64(id) != 4321 {
		t.Fatalf("DeleteRecord must use the RecordId returned by DescribeRecordList, got payload=%+v", (*calls)[1].payload)
	}
}

func TestDeleteRecords_IgnoresMissingRecords(t *testing.T) {
	calls := withTencentCloudAPIFake(t, func(action string, payload map[string]any) string {
		switch action {
		case DescribeRecordList:
			return emptyRecordListResponse()
		case DeleteRecord:
			t.Fatalf("DeleteRecords must not call DeleteRecord when DescribeRecordList finds no matching record")
			return ""
		default:
			t.Fatalf("unexpected Tencent Cloud action %s", action)
			return ""
		}
	})

	provider := &Provider{SecretId: "id", SecretKey: "key"}
	_, err := provider.DeleteRecords(context.Background(), "example.com.", []libdns.Record{testAddressRecord()})
	if err != nil {
		t.Fatalf("DeleteRecords should ignore missing records, got error: %v", err)
	}

	if len(*calls) != 1 || (*calls)[0].action != DescribeRecordList {
		t.Fatalf("DeleteRecords should only look up a missing record, got calls=%+v", *calls)
	}
}
