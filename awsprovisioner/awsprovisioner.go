// The following code is AUTO-GENERATED. Please DO NOT edit.
// To update this generated code, run the following command:
// in the /codegenerator/model subdirectory of this project,
// making sure that `${GOPATH}/bin` is in your `PATH`:
//
// go install && go generate
//
// This package was generated from the schema defined at
// http://references.taskcluster.net/aws-provisioner/v1/api.json

// The AWS Provisioner is responsible for provisioning instances on EC2 for use in
// TaskCluster.  The provisioner maintains a set of worker configurations which
// can be managed with an API that is typically available at
// aws-provisioner.taskcluster.net/v1.  This API can also perform basic instance
// management tasks in addition to maintaining the internal state of worker type
// configuration information.
//
// The Provisioner runs at a configurable interval.  Each iteration of the
// provisioner fetches a current copy the state that the AWS EC2 api reports.  In
// each iteration, we ask the Queue how many tasks are pending for that worker
// type.  Based on the number of tasks pending and the scaling ratio, we may
// submit requests for new instances.  We use pricing information, capacity and
// utility factor information to decide which instance type in which region would
// be the optimal configuration.
//
// Each EC2 instance type will declare a capacity and utility factor.  Capacity is
// the number of tasks that a given machine is capable of running concurrently.
// Utility factor is a relative measure of performance between two instance types.
// We multiply the utility factor by the spot price to compare instance types and
// regions when making the bidding choices.
//
// When a new EC2 instance is instantiated, its user data contains a token in
// `securityToken` that can be used with the `getSecret` method to retrieve
// the worker's credentials and any needed passwords or other restricted
// information.  The worker is responsible for deleting the secret after
// retrieving it, to prevent dissemination of the secret to other proceses
// which can read the instance user data.
//
// See: http://docs.taskcluster.net/aws-provisioner/api-docs
//
// How to use this package
//
// First create an AwsProvisioner object:
//
//  awsProvisioner := awsprovisioner.New("myClientId", "myAccessToken")
//
// and then call one or more of awsProvisioner's methods, e.g.:
//
//  data, callSummary := awsProvisioner.CreateWorkerType(.....)
// handling any errors...
//  if callSummary.Error != nil {
//  	// handle error...
//  }
package awsprovisioner

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/taskcluster/httpbackoff"
	hawk "github.com/tent/hawk-go"
	D "github.com/tj/go-debug"
)

var (
	// Used for logging based on DEBUG environment variable
	// See github.com/tj/go-debug
	debug = D.Debug("awsprovisioner")
)

// apiCall is the generic REST API calling method which performs all REST API
// calls for this library.  Each auto-generated REST API method simply is a
// wrapper around this method, calling it with specific specific arguments.
func (awsProvisioner *AwsProvisioner) apiCall(payload interface{}, method, route string, result interface{}) (interface{}, *CallSummary) {
	callSummary := new(CallSummary)
	callSummary.HttpRequestObject = payload
	var jsonPayload []byte
	jsonPayload, callSummary.Error = json.Marshal(payload)
	if callSummary.Error != nil {
		return result, callSummary
	}
	callSummary.HttpRequestBody = string(jsonPayload)

	httpClient := &http.Client{}

	// function to perform http request - we call this using backoff library to
	// have exponential backoff in case of intermittent failures (e.g. network
	// blips or HTTP 5xx errors)
	httpCall := func() (*http.Response, error, error) {
		var ioReader io.Reader = nil
		if reflect.ValueOf(payload).IsValid() && !reflect.ValueOf(payload).IsNil() {
			ioReader = bytes.NewReader(jsonPayload)
		}
		httpRequest, err := http.NewRequest(method, awsProvisioner.BaseURL+route, ioReader)
		if err != nil {
			return nil, nil, fmt.Errorf("apiCall url cannot be parsed: '%v', is your BaseURL (%v) set correctly?\n%v\n", awsProvisioner.BaseURL+route, awsProvisioner.BaseURL, err)
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		callSummary.HttpRequest = httpRequest
		// Refresh Authorization header with each call...
		// Only authenticate if client library user wishes to.
		if awsProvisioner.Authenticate {
			credentials := &hawk.Credentials{
				ID:   awsProvisioner.ClientId,
				Key:  awsProvisioner.AccessToken,
				Hash: sha256.New,
			}
			reqAuth := hawk.NewRequestAuth(httpRequest, credentials, 0)
			if awsProvisioner.Certificate != "" {
				reqAuth.Ext = base64.StdEncoding.EncodeToString([]byte("{\"certificate\":" + awsProvisioner.Certificate + "}"))
			}
			httpRequest.Header.Set("Authorization", reqAuth.RequestHeader())
		}
		debug("Making http request: %v", httpRequest)
		resp, err := httpClient.Do(httpRequest)
		return resp, err, nil
	}

	// Make HTTP API calls using an exponential backoff algorithm...
	callSummary.HttpResponse, callSummary.Attempts, callSummary.Error = httpbackoff.Retry(httpCall)

	if callSummary.Error != nil {
		return result, callSummary
	}

	// now read response into memory, so that we can return the body
	var body []byte
	body, callSummary.Error = ioutil.ReadAll(callSummary.HttpResponse.Body)

	if callSummary.Error != nil {
		return result, callSummary
	}

	callSummary.HttpResponseBody = string(body)

	// if result is passed in as nil, it means the API defines no response body
	// json
	if reflect.ValueOf(result).IsValid() && !reflect.ValueOf(result).IsNil() {
		callSummary.Error = json.Unmarshal([]byte(callSummary.HttpResponseBody), &result)
		if callSummary.Error != nil {
			// technically not needed since returned outside if, but more comprehensible
			return result, callSummary
		}
	}

	// Return result and callSummary
	return result, callSummary
}

// The entry point into all the functionality in this package is to create an
// AwsProvisioner object.  It contains your authentication credentials, which are
// required for all HTTP operations.
type AwsProvisioner struct {
	// Client ID required by Hawk
	ClientId string
	// Access Token required by Hawk
	AccessToken string
	// The URL of the API endpoint to hit.
	// Use "https://aws-provisioner.taskcluster.net/v1" for production.
	// Please note calling awsprovisioner.New(clientId string, accessToken string) is an
	// alternative way to create an AwsProvisioner object with BaseURL set to production.
	BaseURL string
	// Whether authentication is enabled (e.g. set to 'false' when using taskcluster-proxy)
	// Please note calling awsprovisioner.New(clientId string, accessToken string) is an
	// alternative way to create an AwsProvisioner object with Authenticate set to true.
	Authenticate bool
	// Certificate for temporary credentials
	Certificate string
}

// CallSummary provides information about the underlying http request and
// response issued for a given API call, together with details of any Error
// which occured. After making an API call, be sure to check the returned
// CallSummary.Error - if it is nil, no error occurred.
type CallSummary struct {
	HttpRequest *http.Request
	// Keep a copy of request body in addition to the *http.Request, since
	// accessing the Body via the *http.Request object, you get a io.ReadCloser
	// - and after the request has been made, the body will have been read, and
	// the data lost... This way, it is still available after the api call
	// returns.
	HttpRequestBody string
	// The Go Type which is marshaled into json and used as the http request
	// body.
	HttpRequestObject interface{}
	HttpResponse      *http.Response
	// Keep a copy of response body in addition to the *http.Response, since
	// accessing the Body via the *http.Response object, you get a
	// io.ReadCloser - and after the response has been read once (to unmarshal
	// json into native go types) the data is lost... This way, it is still
	// available after the api call returns.
	HttpResponseBody string
	Error            error
	// Keep a record of how many http requests were attempted
	Attempts int
}

// Returns a pointer to AwsProvisioner, configured to run against production.  If you
// wish to point at a different API endpoint url, set BaseURL to the preferred
// url. Authentication can be disabled (for example if you wish to use the
// taskcluster-proxy) by setting Authenticate to false.
//
// For example:
//  awsProvisioner := awsprovisioner.New("123", "456")                       // set clientId and accessToken
//  awsProvisioner.Authenticate = false                                      // disable authentication (true by default)
//  awsProvisioner.BaseURL = "http://localhost:1234/api/AwsProvisioner/v1"   // alternative API endpoint (production by default)
//  data, callSummary := awsProvisioner.CreateWorkerType(.....)              // for example, call the CreateWorkerType(.....) API endpoint (described further down)...
//  if callSummary.Error != nil {
//  	// handle errors...
//  }
func New(clientId string, accessToken string) *AwsProvisioner {
	return &AwsProvisioner{
		ClientId:     clientId,
		AccessToken:  accessToken,
		BaseURL:      "https://aws-provisioner.taskcluster.net/v1",
		Authenticate: true,
	}
}

// Create a worker type.  A worker type contains all the configuration
// needed for the provisioner to manage the instances.  Each worker type
// knows which regions and which instance types are allowed for that
// worker type.  Remember that Capacity is the number of concurrent tasks
// that can be run on a given EC2 resource and that Utility is the relative
// performance rate between different instance types.  There is no way to
// configure different regions to have different sets of instance types
// so ensure that all instance types are available in all regions.
// This function is idempotent.
//
// Once a worker type is in the provisioner, a back ground process will
// begin creating instances for it based on its capacity bounds and its
// pending task count from the Queue.  It is the worker's responsibility
// to shut itself down.  The provisioner has a limit (currently 96hours)
// for all instances to prevent zombie instances from running indefinitely.
//
// The provisioner will ensure that all instances created are tagged with
// aws resource tags containing the provisioner id and the worker type.
//
// If provided, the secrets in the global, region and instance type sections
// are available using the secrets api.  If specified, the scopes provided
// will be used to generate a set of temporary credentials available with
// the other secrets.
//
// Required scopes:
//   * aws-provisioner:manage-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#createWorkerType
func (awsProvisioner *AwsProvisioner) CreateWorkerType(workerType string, payload *CreateWorkerTypeRequest) (*GetWorkerTypeRequest, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(payload, "PUT", "/worker-type/"+url.QueryEscape(workerType), new(GetWorkerTypeRequest))
	return responseObject.(*GetWorkerTypeRequest), callSummary
}

// Provide a new copy of a worker type to replace the existing one.
// This will overwrite the existing worker type definition if there
// is already a worker type of that name.  This method will return a
// 200 response along with a copy of the worker type definition created
// Note that if you are using the result of a GET on the worker-type
// end point that you will need to delete the lastModified and workerType
// keys from the object returned, since those fields are not allowed
// the request body for this method
//
// Otherwise, all input requirements and actions are the same as the
// create method.
//
// Required scopes:
//   * aws-provisioner:manage-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#updateWorkerType
func (awsProvisioner *AwsProvisioner) UpdateWorkerType(workerType string, payload *CreateWorkerTypeRequest) (*GetWorkerTypeRequest, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(payload, "POST", "/worker-type/"+url.QueryEscape(workerType)+"/update", new(GetWorkerTypeRequest))
	return responseObject.(*GetWorkerTypeRequest), callSummary
}

// Retreive a copy of the requested worker type definition.
// This copy contains a lastModified field as well as the worker
// type name.  As such, it will require manipulation to be able to
// use the results of this method to submit date to the update
// method.
//
// Required scopes:
//   * aws-provisioner:view-worker-type:<workerType>, or
//   * aws-provisioner:manage-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#workerType
func (awsProvisioner *AwsProvisioner) WorkerType(workerType string) (*GetWorkerTypeRequest, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(nil, "GET", "/worker-type/"+url.QueryEscape(workerType), new(GetWorkerTypeRequest))
	return responseObject.(*GetWorkerTypeRequest), callSummary
}

// Delete a worker type definition.  This method will only delete
// the worker type definition from the storage table.  The actual
// deletion will be handled by a background worker.  As soon as this
// method is called for a worker type, the background worker will
// immediately submit requests to cancel all spot requests for this
// worker type as well as killing all instances regardless of their
// state.  If you want to gracefully remove a worker type, you must
// either ensure that no tasks are created with that worker type name
// or you could theoretically set maxCapacity to 0, though, this is
// not a supported or tested action
//
// Required scopes:
//   * aws-provisioner:manage-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#removeWorkerType
func (awsProvisioner *AwsProvisioner) RemoveWorkerType(workerType string) *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "DELETE", "/worker-type/"+url.QueryEscape(workerType), nil)
	return callSummary
}

// Return a list of string worker type names.  These are the names
// of all managed worker types known to the provisioner.  This does
// not include worker types which are left overs from a deleted worker
// type definition but are still running in AWS.
//
// Required scopes:
//   * aws-provisioner:list-worker-types
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#listWorkerTypes
func (awsProvisioner *AwsProvisioner) ListWorkerTypes() (*ListWorkerTypes, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(nil, "GET", "/list-worker-types", new(ListWorkerTypes))
	return responseObject.(*ListWorkerTypes), callSummary
}

// Insert a secret into the secret storage.  The supplied secrets will
// be provided verbatime via `getSecret`, while the supplied scopes will
// be converted into credentials by `getSecret`.
//
// This method is not ordinarily used in production; instead, the provisioner
// creates a new secret directly for each spot bid.
//
// Required scopes:
//   * aws-provisioner:create-secret
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#createSecret
func (awsProvisioner *AwsProvisioner) CreateSecret(token string, payload *GetSecretRequest) *CallSummary {
	_, callSummary := awsProvisioner.apiCall(payload, "PUT", "/secret/"+url.QueryEscape(token), nil)
	return callSummary
}

// Retrieve a secret from storage.  The result contains any passwords or
// other restricted information verbatim as well as a temporary credential
// based on the scopes specified when the secret was created.
//
// It is important that this secret is deleted by the consumer (`removeSecret`),
// or else the secrets will be visible to any process which can access the
// user data associated with the instance.
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#getSecret
func (awsProvisioner *AwsProvisioner) GetSecret(token string) (*GetSecretResponse, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(nil, "GET", "/secret/"+url.QueryEscape(token), new(GetSecretResponse))
	return responseObject.(*GetSecretResponse), callSummary
}

// An instance will report in by giving its instance id as well
// as its security token.  The token is given and checked to ensure
// that it matches a real token that exists to ensure that random
// machines do not check in.  We could generate a different token
// but that seems like overkill
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#instanceStarted
func (awsProvisioner *AwsProvisioner) InstanceStarted(instanceId string, token string) *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "GET", "/instance-started/"+url.QueryEscape(instanceId)+"/"+url.QueryEscape(token), nil)
	return callSummary
}

// Remove a secret.  After this call, a call to `getSecret` with the given
// token will return no information.
//
// It is very important that the consumer of a
// secret delete the secret from storage before handing over control
// to untrusted processes to prevent credential and/or secret leakage.
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#removeSecret
func (awsProvisioner *AwsProvisioner) RemoveSecret(token string) *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "DELETE", "/secret/"+url.QueryEscape(token), nil)
	return callSummary
}

// This method returns a preview of all possible launch specifications
// that this worker type definition could submit to EC2.  It is used to
// test worker types, nothing more
//
// **This API end-point is experimental and may be subject to change without warning.**
//
// Required scopes:
//   * aws-provisioner:view-worker-type:<workerType>, or
//   * aws-provisioner:manage-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#getLaunchSpecs
func (awsProvisioner *AwsProvisioner) GetLaunchSpecs(workerType string) (*GetAllLaunchSpecsResponse, *CallSummary) {
	responseObject, callSummary := awsProvisioner.apiCall(nil, "GET", "/worker-type/"+url.QueryEscape(workerType)+"/launch-specifications", new(GetAllLaunchSpecsResponse))
	return responseObject.(*GetAllLaunchSpecsResponse), callSummary
}

// This method is a left over and will be removed as soon as the
// tools.tc.net UI is updated to use the per-worker state
//
// **DEPRECATED.**
//
// Required scopes:
//   * aws-provisioner:aws-state
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#awsState
func (awsProvisioner *AwsProvisioner) AwsState() *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "GET", "/aws-state", nil)
	return callSummary
}

// Return the state of a given workertype as stored by the provisioner.
// This state is stored as three lists: 1 for all instances, 1 for requests
// which show in the ec2 api and 1 list for those only tracked internally
// in the provisioner.
//
// Required scopes:
//   * aws-provisioner:view-worker-type:<workerType>
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#state
func (awsProvisioner *AwsProvisioner) State(workerType string) *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "GET", "/state/"+url.QueryEscape(workerType), nil)
	return callSummary
}

// Documented later...
//
// **Warning** this api end-point is **not stable**.
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#ping
func (awsProvisioner *AwsProvisioner) Ping() *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "GET", "/ping", nil)
	return callSummary
}

// Get an API reference!
//
// **Warning** this api end-point is **not stable**.
//
// See http://docs.taskcluster.net/aws-provisioner/api-docs/#apiReference
func (awsProvisioner *AwsProvisioner) ApiReference() *CallSummary {
	_, callSummary := awsProvisioner.apiCall(nil, "GET", "/api-reference", nil)
	return callSummary
}

type (
	// A Secret
	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/create-secret-request.json#
	GetSecretRequest struct {
		// The date at which the secret is no longer guarunteed to exist
		Expiration Time `json:"expiration"`
		// List of strings which are scopes for temporary credentials to give
		// to the worker through the secret system.  Scopes must be composed of
		// printable ASCII characters and spaces.
		Scopes []string `json:"scopes"`
		// Free form object which contains the secrets stored
		Secrets json.RawMessage `json:"secrets"`
		// A Slug ID which is the uniquely addressable token to access this
		// set of secrets
		Token string `json:"token"`
		// A string describing what the secret will be used for
		WorkerType string `json:"workerType"`
	}

	// A worker launchSpecification and required metadata
	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/create-worker-type-request.json#
	CreateWorkerTypeRequest struct {
		// True if this worker type is allowed on demand instances.  Currently
		// ignored
		CanUseOndemand bool `json:"canUseOndemand"`
		// True if this worker type is allowed spot instances.  Currently ignored
		// as all instances are Spot
		CanUseSpot    bool `json:"canUseSpot"`
		InstanceTypes []struct {
			// This number represents the number of tasks that this instance type
			// is capable of running concurrently.  This is used by the provisioner
			// to know how many pending tasks to offset a pending instance of this
			// type by
			Capacity int `json:"capacity"`
			// InstanceType name for Amazon.
			InstanceType string `json:"instanceType"`
			// LaunchSpecification entries unique to this InstanceType
			LaunchSpec json.RawMessage `json:"launchSpec"`
			// Scopes which should be included for this InstanceType.  Scopes must
			// be composed of printable ASCII characters and spaces.
			Scopes []string `json:"scopes"`
			// Static Secrets unique to this InstanceType
			Secrets json.RawMessage `json:"secrets"`
			// UserData entries unique to this InstanceType
			UserData json.RawMessage `json:"userData"`
			// This number is a relative measure of performance between two instance
			// types.  It is multiplied by the spot price from Amazon to figure out
			// which instance type is the cheapest one
			Utility int `json:"utility"`
		} `json:"instanceTypes"`
		// Launch Specification entries which are used in all regions and all instance types
		LaunchSpec json.RawMessage `json:"launchSpec"`
		// Maximum number of capacity units to be provisioned.
		MaxCapacity int `json:"maxCapacity"`
		// Maximum price we'll pay.  Like minPrice, this takes into account the
		// utility factor when figuring out what the actual SpotPrice submitted
		// to Amazon will be
		MaxPrice int `json:"maxPrice"`
		// Minimum number of capacity units to be provisioned.  A capacity unit
		// is an abstract unit of capacity, where one capacity unit is roughly
		// one task which should be taken off the queue
		MinCapacity int `json:"minCapacity"`
		// Minimum price to pay for an instance.  A Price is considered to be the
		// Amazon Spot Price multiplied by the utility factor of the InstantType
		// as specified in the instanceTypes list.  For example, if the minPrice
		// is set to $0.5 and the utility factor is 2, the actual minimum bid
		// used will be $0.25
		MinPrice int `json:"minPrice"`
		Regions  []struct {
			// LaunchSpecification entries unique to this Region
			LaunchSpec struct {
				// Per-region AMI ImageId
				ImageId string `json:"ImageId"`
			} `json:"launchSpec"`
			// The Amazon AWS Region being configured.  Example: us-west-1
			Region string `json:"region"`
			// Scopes which should be included for this Region.  Scopes must be
			// composed of printable ASCII characters and spaces.
			Scopes []string `json:"scopes"`
			// Static Secrets unique to this Region
			Secrets json.RawMessage `json:"secrets"`
			// UserData entries unique to this Region
			UserData json.RawMessage `json:"userData"`
		} `json:"regions"`
		// A scaling ratio of `0.2` means that the provisioner will attempt to keep
		// the number of pending tasks around 20% of the provisioned capacity.
		// This results in pending tasks waiting 20% of the average task execution
		// time before starting to run.
		// A higher scaling ratio often results in better utilization and longer
		// waiting times. For workerTypes running long tasks a short scaling ratio
		// may be prefered, but for workerTypes running quick tasks a higher scaling
		// ratio may increase utilization without major delays.
		// If using a scaling ratio of 0, the provisioner will attempt to keep the
		// capacity of pending spot requests equal to the number of pending tasks.
		ScalingRatio int `json:"scalingRatio"`
		// Scopes to issue credentials to for all regions Scopes must be composed of
		// printable ASCII characters and spaces.
		Scopes []string `json:"scopes"`
		// Static secrets entries which are used in all regions and all instance types
		Secrets json.RawMessage `json:"secrets"`
		// UserData entries which are used in all regions and all instance types
		UserData json.RawMessage `json:"userData"`
	}

	// All of the launch specifications for a worker type
	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/get-launch-specs-response.json#
	GetAllLaunchSpecsResponse json.RawMessage

	// Secrets from the provisioner
	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/get-secret-response.json#
	GetSecretResponse struct {
		// Generated Temporary credentials from the Provisioner
		Credentials struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientId    string `json:"clientId"`
		} `json:"credentials"`
		// Free-form object which contains secrets from the worker type definition
		Data json.RawMessage `json:"data"`
	}

	// A worker launchSpecification and required metadata
	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/get-worker-type-response.json#
	GetWorkerTypeRequest struct {
		// True if this worker type is allowed on demand instances.  Currently
		// ignored
		CanUseOndemand bool `json:"canUseOndemand"`
		// True if this worker type is allowed spot instances.  Currently ignored
		// as all instances are Spot
		CanUseSpot    bool `json:"canUseSpot"`
		InstanceTypes []struct {
			// This number represents the number of tasks that this instance type
			// is capable of running concurrently.  This is used by the provisioner
			// to know how many pending tasks to offset a pending instance of this
			// type by
			Capacity int `json:"capacity"`
			// InstanceType name for Amazon.
			InstanceType string `json:"instanceType"`
			// LaunchSpecification entries unique to this InstanceType
			LaunchSpec json.RawMessage `json:"launchSpec"`
			// Scopes which should be included for this InstanceType.  Scopes must
			// be composed of printable ASCII characters and spaces.
			Scopes []string `json:"scopes"`
			// Static Secrets unique to this InstanceType
			Secrets json.RawMessage `json:"secrets"`
			// UserData entries unique to this InstanceType
			UserData json.RawMessage `json:"userData"`
			// This number is a relative measure of performance between two instance
			// types.  It is multiplied by the spot price from Amazon to figure out
			// which instance type is the cheapest one
			Utility int `json:"utility"`
		} `json:"instanceTypes"`
		// ISO Date string (e.g. new Date().toISOString()) which represents the time
		// when this worker type definition was last altered (inclusive of creation)
		LastModified Time `json:"lastModified"`
		// Launch Specification entries which are used in all regions and all instance types
		LaunchSpec json.RawMessage `json:"launchSpec"`
		// Maximum number of capacity units to be provisioned.
		MaxCapacity int `json:"maxCapacity"`
		// Maximum price we'll pay.  Like minPrice, this takes into account the
		// utility factor when figuring out what the actual SpotPrice submitted
		// to Amazon will be
		MaxPrice int `json:"maxPrice"`
		// Minimum number of capacity units to be provisioned.  A capacity unit
		// is an abstract unit of capacity, where one capacity unit is roughly
		// one task which should be taken off the queue
		MinCapacity int `json:"minCapacity"`
		// Minimum price to pay for an instance.  A Price is considered to be the
		// Amazon Spot Price multiplied by the utility factor of the InstantType
		// as specified in the instanceTypes list.  For example, if the minPrice
		// is set to $0.5 and the utility factor is 2, the actual minimum bid
		// used will be $0.25
		MinPrice int `json:"minPrice"`
		Regions  []struct {
			// LaunchSpecification entries unique to this Region
			LaunchSpec struct {
				// Per-region AMI ImageId
				ImageId string `json:"ImageId"`
			} `json:"launchSpec"`
			// The Amazon AWS Region being configured.  Example: us-west-1
			Region string `json:"region"`
			// Scopes which should be included for this Region.  Scopes must be
			// composed of printable ASCII characters and spaces.
			Scopes []string `json:"scopes"`
			// Static Secrets unique to this Region
			Secrets json.RawMessage `json:"secrets"`
			// UserData entries unique to this Region
			UserData json.RawMessage `json:"userData"`
		} `json:"regions"`
		// A scaling ratio of `0.2` means that the provisioner will attempt to keep
		// the number of pending tasks around 20% of the provisioned capacity.
		// This results in pending tasks waiting 20% of the average task execution
		// time before starting to run.
		// A higher scaling ratio often results in better utilization and longer
		// waiting times. For workerTypes running long tasks a short scaling ratio
		// may be prefered, but for workerTypes running quick tasks a higher scaling
		// ratio may increase utilization without major delays.
		// If using a scaling ratio of 0, the provisioner will attempt to keep the
		// capacity of pending spot requests equal to the number of pending tasks.
		ScalingRatio int `json:"scalingRatio"`
		// Scopes to issue credentials to for all regions.  Scopes must be composed
		// of printable ASCII characters and spaces.
		Scopes []string `json:"scopes"`
		// Static secrets entries which are used in all regions and all instance types
		Secrets json.RawMessage `json:"secrets"`
		// UserData entries which are used in all regions and all instance types
		UserData json.RawMessage `json:"userData"`
		// The ID of the workerType
		//
		// Syntax: ^[A-Za-z0-9+/=_-]{1,22}$
		WorkerType string `json:"workerType"`
	}

	//
	// See http://schemas.taskcluster.net/aws-provisioner/v1/list-worker-types-response.json#
	ListWorkerTypes []string
)

// MarshalJSON calls json.RawMessage method of the same name. Required since
// GetAllLaunchSpecsResponse is of type json.RawMessage...
func (this *GetAllLaunchSpecsResponse) MarshalJSON() ([]byte, error) {
	x := json.RawMessage(*this)
	return (&x).MarshalJSON()
}

// UnmarshalJSON is a copy of the json.RawMessage implementation.
func (this *GetAllLaunchSpecsResponse) UnmarshalJSON(data []byte) error {
	if this == nil {
		return errors.New("GetAllLaunchSpecsResponse: UnmarshalJSON on nil pointer")
	}
	*this = append((*this)[0:0], data...)
	return nil
}

// Wraps time.Time in order that json serialisation/deserialisation can be adapted.
// Marshaling time.Time types results in RFC3339 dates with nanosecond precision
// in the user's timezone. In order that the json date representation is consistent
// between what we send in json payloads, and what taskcluster services return,
// we wrap time.Time into type awsprovisioner.Time which marshals instead
// to the same format used by the TaskCluster services; UTC based, with millisecond
// precision, using 'Z' timezone, e.g. 2015-10-27T20:36:19.255Z.
type Time time.Time

// MarshalJSON implements the json.Marshaler interface.
// The time is a quoted string in RFC 3339 format, with sub-second precision added if present.
func (t Time) MarshalJSON() ([]byte, error) {
	if y := time.Time(t).Year(); y < 0 || y >= 10000 {
		// RFC 3339 is clear that years are 4 digits exactly.
		// See golang.org/issue/4556#c15 for more discussion.
		return nil, errors.New("queue.Time.MarshalJSON: year outside of range [0,9999]")
	}
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The time is expected to be a quoted string in RFC 3339 format.
func (t *Time) UnmarshalJSON(data []byte) (err error) {
	// Fractional seconds are handled implicitly by Parse.
	x := new(time.Time)
	*x, err = time.Parse(`"`+time.RFC3339+`"`, string(data))
	*t = Time(*x)
	return
}

// Returns the Time in canonical RFC3339 representation, e.g.
// 2015-10-27T20:36:19.255Z
func (t Time) String() string {
	return time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z")
}
