package examples

import (
	"context"
	"fmt"
	"net/http"

	"github.com/georgebent/go-httpclient/gohttp"
)

func Post(url string) (string, error) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer ABC-12345678")

	body := make(map[string]string)
	body["firstname"] = "John"
	body["lastname"] = "Stranger"
	body["type"] = "Builder Singleton"

	response, err := HttpClient.Post(
		context.Background(),
		url,
		gohttp.WithHeaders(headers),
		gohttp.WithBody(body),
	)
	if err != nil {
		fmt.Println(err.Error())

		return "", err
	}

	fmt.Println(response.StatusCode)

	return response.BodyString(), nil
}
