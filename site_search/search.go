package main

import (
	"encoding/json"
	"fmt"

	"github.com/gocolly/colly"
)

func main() {

	c := colly.NewCollector()
	c2 := c.Clone()

	var token string

	payload := []byte(`{"operationName":"Login","variables":{},"query":"mutation Login($forceCtLogin: Boolean, $forceCtExpiry: Boolean) {\n  login(forceCtLogin: $forceCtLogin, forceCtExpiry: $forceCtExpiry)\n}"}`)

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
		r.Headers.Set("Content-Type", "application/json;charset=UTF-8")
	})

	c2.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
		fmt.Println("Token: ", token)
		r.Headers.Set("Content-Type", "application/json;charset=UTF-8")
		r.Headers.Set("Authorization", token)
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)

		var data map[string]interface{}

		err := json.Unmarshal([]byte(r.Body), &data)
		if err != nil {
			fmt.Println(err)
		}

		key := "data"
		value, ok := data[key]
		if !ok {
			fmt.Printf("key %q not found in data\n", key)
			return
		}

		dataMap, ok := value.(map[string]interface{})
		if !ok {
			fmt.Printf("value for key %q is not a map[string]interface{}\n", key)
			return
		}

		fmt.Println(dataMap["login"])
		token = dataMap["login"].(string)
		payload = []byte(`{"operationName":"PickABrickQuery","variables":{"input":{}},"query":"query PickABrickQuery($input: ElementQueryArgs, $sku: String) {\n  __typename\n  elements(input: $input) {\n    count\n    facets {\n      ...FacetData\n      __typename\n    }\n    sortOptions {\n      ...Sort_SortOptions\n      __typename\n    }\n    results {\n      ...ElementLeafData\n      __typename\n    }\n    set {\n      id\n      type\n      name\n      imageUrl\n      instructionsUrl\n      pieces\n      inStock\n      price {\n        formattedAmount\n        __typename\n      }\n      __typename\n    }\n    total\n    __typename\n  }\n}\n\nfragment FacetData on Facet {\n  id\n  key\n  name\n  labels {\n    count\n    key\n    name\n    children {\n      count\n      key\n      name\n      ... on FacetValue {\n        value\n        __typename\n      }\n      __typename\n    }\n    ... on FacetValue {\n      value\n      __typename\n    }\n    ... on FacetRange {\n      from\n      to\n      __typename\n    }\n    __typename\n  }\n  __typename\n}\n\nfragment Sort_SortOptions on SortOptions {\n  id\n  key\n  direction\n  label\n  analyticLabel\n  __typename\n}\n\nfragment ElementLeafData on Element {\n  id\n  name\n  categories {\n    name\n    key\n    __typename\n  }\n  inStock\n  ... on SingleVariantElement {\n    variant {\n      ...ElementLeafVariant\n      __typename\n    }\n    __typename\n  }\n  ... on MultiVariantElement {\n    variants {\n      ...ElementLeafVariant\n      __typename\n    }\n    __typename\n  }\n  __typename\n}\n\nfragment ElementLeafVariant on ElementVariant {\n  id\n  price {\n    centAmount\n    formattedAmount\n    __typename\n  }\n  attributes {\n    designNumber\n    colourId\n    deliveryChannel\n    maxOrderQuantity\n    system\n    quantityInSet(sku: $sku)\n    indexImageURL\n    __typename\n  }\n  __typename\n}"}`)
		c2.PostRaw("https://www.lego.com/api/graphql/PickABrickQuery", payload)
	})

	c2.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)
		fmt.Println(string(r.Body))
	})

	c.OnError(func(r *colly.Response, e error) {
		fmt.Println("Got this error:", e)
	})

	c2.OnError(func(r *colly.Response, e error) {
		fmt.Println("Got this error:", e)
	})

	c.PostRaw("https://www.lego.com/api/graphql/Login", payload)
}
