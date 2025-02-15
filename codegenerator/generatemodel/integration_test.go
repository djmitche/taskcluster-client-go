package main

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/index"
	"github.com/taskcluster/taskcluster-client-go/queue"
)

// This is a silly test that looks for the latest mozilla-central buildbot linux64 l10n build
// and asserts that it must have a created time between a year ago and an hour in the future.
//
// Could easily break at a point in the future, at which point we can change to something else.
//
// Note, no credentials are needed, so this can be run even on travis-ci.org, for example.
func TestFindLatestBuildbotTask(t *testing.T) {
	Index := index.New("", "")
	Queue := queue.New("", "")
	itr, cs1 := Index.FindTask("buildbot.branches.mozilla-central.linux64.l10n")
	if cs1.Error != nil {
		t.Fatalf("%v\n", cs1.Error)
	}
	taskId := itr.TaskId
	td, cs2 := Queue.Task(taskId)
	if cs2.Error != nil {
		t.Fatalf("%v\n", cs2.Error)
	}
	created := time.Time(td.Created).Local()

	// calculate time an hour in the future to allow for clock drift
	now := time.Now().Local()
	inAnHour := now.Add(time.Hour * 1)
	aYearAgo := now.AddDate(-1, 0, 0)
	t.Log("")
	t.Log("  => Task " + taskId + " was created on " + created.Format("Mon, 2 Jan 2006 at 15:04:00 -0700"))
	t.Log("")
	if created.After(inAnHour) {
		t.Log("Current time: " + now.Format("Mon, 2 Jan 2006 at 15:04:00 -0700"))
		t.Error("Task " + taskId + " has a creation date that is over an hour in the future")
	}
	if created.Before(aYearAgo) {
		t.Log("Current time: " + now.Format("Mon, 2 Jan 2006 at 15:04:00 -0700"))
		t.Error("Task " + taskId + " has a creation date that is over a year old")
	}

}

// Tests whether it is possible to define a task against the production Queue.
func TestDefineTask(t *testing.T) {
	clientId := os.Getenv("TASKCLUSTER_CLIENT_ID")
	accessToken := os.Getenv("TASKCLUSTER_ACCESS_TOKEN")
	certificate := os.Getenv("TASKCLUSTER_CERTIFICATE")
	if clientId == "" || accessToken == "" {
		t.Skip("Skipping test TestDefineTask since TASKCLUSTER_CLIENT_ID and/or TASKCLUSTER_ACCESS_TOKEN env vars not set")
	}
	myQueue := queue.New(clientId, accessToken)
	myQueue.Certificate = certificate

	taskId := slugid.Nice()
	created := time.Now()
	deadline := created.AddDate(0, 0, 1)
	expires := deadline

	td := &queue.TaskDefinition{
		Created:  queue.Time(created),
		Deadline: queue.Time(deadline),
		Expires:  queue.Time(expires),
		Extra:    json.RawMessage(`{"index":{"rank":12345}}`),
		Metadata: struct {
			Description string `json:"description"`
			Name        string `json:"name"`
			Owner       string `json:"owner"`
			Source      string `json:"source"`
		}{
			Description: "Stuff",
			Name:        "[TC] Pete",
			Owner:       "pmoore@mozilla.com",
			Source:      "http://everywhere.com/",
		},
		Payload:       json.RawMessage(`{"features":{"relengApiProxy":true}}`),
		ProvisionerId: "win-provisioner",
		Retries:       5,
		Routes: []string{
			"tc-treeherder.mozilla-inbound.bcf29c305519d6e120b2e4d3b8aa33baaf5f0163",
			"tc-treeherder-stage.mozilla-inbound.bcf29c305519d6e120b2e4d3b8aa33baaf5f0163",
		},
		SchedulerId: "go-test-test-scheduler",
		Scopes: []string{
			"test-worker:image:toastposter/pumpkin:0.5.6",
		},
		Tags:        json.RawMessage(`{"createdForUser":"cbook@mozilla.com"}`),
		Priority:    json.RawMessage(`"high"`),
		TaskGroupId: "dtwuF2n9S-i83G37V9eBuQ",
		WorkerType:  "win2008-worker",
	}

	tsr, cs := myQueue.DefineTask(taskId, td)

	//////////////////////////////////
	// And now validate results.... //
	//////////////////////////////////

	if cs.Error != nil {
		b := bytes.Buffer{}
		cs.HttpRequest.Header.Write(&b)
		headers := regexp.MustCompile(`(mac|nonce)="[^"]*"`).ReplaceAllString(b.String(), `$1="***********"`)
		t.Logf("\n\nRequest sent:\n\nURL: %s\nMethod: %s\nHeaders:\n%v\nBody: %s", cs.HttpRequest.URL, cs.HttpRequest.Method, headers, cs.HttpRequestBody)
		t.Fatalf("\n\nResponse received:\n\n%s", cs.Error)
	}

	t.Logf("Task https://queue.taskcluster.net/v1/task/%v created successfully", taskId)

	if provisionerId := cs.HttpRequestObject.(*queue.TaskDefinition).ProvisionerId; provisionerId != "win-provisioner" {
		t.Errorf("provisionerId 'win-provisioner' expected but got %s", provisionerId)
	}
	if schedulerId := tsr.Status.SchedulerId; schedulerId != "go-test-test-scheduler" {
		t.Errorf("schedulerId 'go-test-test-scheduler' expected but got %s", schedulerId)
	}
	if retriesLeft := tsr.Status.RetriesLeft; retriesLeft != 5 {
		t.Errorf("Expected 'retriesLeft' to be 5, but got %v", retriesLeft)
	}
	if state := tsr.Status.State; state != "unscheduled" {
		t.Errorf("Expected 'state' to be 'unscheduled', but got %s", state)
	}
	submittedPayload := cs.HttpRequestBody

	// only the contents is relevant below - the formatting and order of properties does not matter
	// since a json comparison is done, not a string comparison...
	expectedJson := []byte(`
	{
	  "created":  "` + created.UTC().Format("2006-01-02T15:04:05.000Z") + `",
	  "deadline": "` + deadline.UTC().Format("2006-01-02T15:04:05.000Z") + `",
	  "expires":  "` + expires.UTC().Format("2006-01-02T15:04:05.000Z") + `",

	  "taskGroupId": "dtwuF2n9S-i83G37V9eBuQ",
	  "workerType":  "win2008-worker",
	  "schedulerId": "go-test-test-scheduler",

	  "payload": {
	    "features": {
	      "relengApiProxy":true
	    }
	  },

	  "priority":      "high",
	  "provisionerId": "win-provisioner",
	  "retries":       5,

	  "routes": [
	    "tc-treeherder.mozilla-inbound.bcf29c305519d6e120b2e4d3b8aa33baaf5f0163",
	    "tc-treeherder-stage.mozilla-inbound.bcf29c305519d6e120b2e4d3b8aa33baaf5f0163"
	  ],

	  "scopes": [
	    "test-worker:image:toastposter/pumpkin:0.5.6"
	  ],

	  "tags": {
	    "createdForUser": "cbook@mozilla.com"
	  },

	  "extra": {
	    "index": {
	      "rank": 12345
	    }
	  },

	  "metadata": {
	    "description": "Stuff",
	    "name":        "[TC] Pete",
	    "owner":       "pmoore@mozilla.com",
	    "source":      "http://everywhere.com/"
	  }
	}
	`)

	jsonCorrect, formattedExpected, formattedActual, err := jsonEqual(expectedJson, []byte(submittedPayload))
	if err != nil {
		t.Fatalf("Exception thrown formatting json data!\n%s\n\nStruggled to format either:\n%s\n\nor:\n\n%s", err, string(expectedJson), submittedPayload)
	}

	if !jsonCorrect {
		t.Log("Anticipated json not generated. Expected:")
		t.Logf("%s", formattedExpected)
		t.Log("Actual:")
		t.Errorf("%s", formattedActual)
	}
}

// Checks whether two json []byte are equivalent (equal) by formatting/ordering
// both of them consistently, and then comparing if formatted versions are
// identical. Returns true/false together with formatted json, and any error.
func jsonEqual(a []byte, b []byte) (bool, []byte, []byte, error) {
	a_, err := formatJson(a)
	if err != nil {
		return false, nil, nil, err
	}
	b_, err := formatJson(b)
	if err != nil {
		return false, a_, nil, err
	}
	return string(a_) == string(b_), a_, b_, nil
}

// Takes json []byte input, unmarshals and then marshals, in order to get a
// canonical representation of json (i.e. formatted with objects ordered).
// Ugly and perhaps inefficient, but effective! :p
func formatJson(a []byte) ([]byte, error) {
	tmpObj := new(interface{})
	err := json.Unmarshal(a, &tmpObj)
	if err != nil {
		return a, err
	}
	return json.MarshalIndent(&tmpObj, "", "  ")
}
