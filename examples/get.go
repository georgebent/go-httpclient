package examples

import "context"

func Get() (string, error) {
	res, err := HttpClient.Get(context.Background(), "http://localhost")
	if err != nil {
		return "", err
	}

	return res.BodyString(), nil
}
