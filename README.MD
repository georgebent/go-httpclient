# Go HTTP Client

A production-ready HTTP client in Go with lots of useful features and using nothing more than the standard library of the language.

## Installation

```bash
# Go Modules
require github.com/georgebent/go-httpclient
```

## Usage
In order to use the library for making HTTP calls you need to import the corresponding HTTP package:

```go
import "github.com/georgebent/go-httpclient/gohttp"
```

## Configuring the client
Once you have imported the package, you can now start using the client. First you need to configure and build the client as you need:

```go
headers := make(http.Header)
headers.Set("Some-Common-Header", "value-for-all-requests")

// Create a new builder:
client := gohttp.NewBuilder().

// You can set global headers to be used in every request made by this client:
SetHeaders(headers).

// Configure the timeout for getting a new connection:
SetConnectionTimeout(2 * time.Second).

// Configure the timeout for performing the actual HTTP call:
SetResponseTimeout(3 * time.Second).

// Configure the User-Agent header that will be used for all of the requests:
SetUserAgent("Your-User-Agent").

// Finally, build the client and start using it!
Build()
```

## Performing HTTP calls
The ``Client`` interface provides convenient methods that you can use to perform different HTTP calls. If you get an error then you can safely ignore the response object since it won't be there.

**Important:** There is no need to read & close anything from the response since the client is doing all of this for you. You just need to get the response and start using it!

Take a look at all of the [EXAMPLES](examples) for more information.

### Get

```go
res, err := HttpClient.Get("http://localhost", nil)
if err != nil {
	return "", err
}

fmt.Println(response.StatusCode)
fmt.Println(response.BodyString())
```

### Post

```go

// The struct representing the actual response from the API we're calling:
headers := make(http.Header)
headers.Set("Authorization", "Bearer ABC-12345678")

body := make(map[string]string)
body["firstname"] = "John"
body["lastname"] = "Stranger"
body["type"] = "Builder"

response, error := HttpClient.Post("http://localhost", headers, body)
if error != nil {
	fmt.Println(string(error.Error()))

	return "", error
}

fmt.Println(response.StatusCode)
fmt.Println(response.BodyString())

```

## Testing

The library provides a convenient package for mocking requests and getting a particular response. The mock key is generated using the ``HTTP method``, the ``request URL`` and the ``request body``. Every request with these same elements will return the same mock.

In order to use the mocking features you need to import the corresponding package:

```go
import "github.com/georgebent/go-httpclient/gohttp"
```

### Starting the mock server:
```go
func TestMain(m *testing.M) {
	// Tell the HTTP library to mock any further requests from here.
	gohttp_mock.MockupServer.StartMockServer()

	// Start the test cases for this pacakge:
	os.Exit(m.Run())
}
```

Once you start the mock server, every request will be handled by this server and will not be sent against the real API. If there is no mock matching the current request you'll get an error saying ``no mock matching {METHOD} from '{URL}' with given body``.

### Configuring a given HTTP mock:

```go
// Delete all mocks in every new test case to ensure a clean environment:
gohttp_mock.MockupServer.DeleteMocks()

// Configure a new mock:
gohttp_mock.MockupServer.AddMock(gohttp_mock.Mock{
	Method:      http.MethodPost,
	Url:         "https://api.github.com/user/repos",
	RequestBody: `{"name":"test-repo","private":true}`,

	Error: errors.New("timeout from github"),
})
```

## Linting

The project uses `golangci-lint` with a focused ruleset aimed at catching bugs and keeping formatting consistent without adding noisy style-only checks.

```bash
env GOCACHE=$(pwd)/.gocache GOLANGCI_LINT_CACHE=$(pwd)/.golangci-cache golangci-lint run ./...
```

## Collaboration

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
