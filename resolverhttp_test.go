package autodochttprouter

import (
	"fmt"
	"net/http"
	"testing"
)

func TestComment(t *testing.T) {
	type R2 struct {
		SID     int64  `json:"id"`
		SubName string `json:"name" comment:"второе имя"`
	}

	type R3 struct {
		SID     int64  `json:"id"`
		SubName string `json:"name" comment:"третье имя"`
	}

	type R4 struct {
		SID     int64  `json:"id"`
		SubName string `json:"name" comment:"четвертое имя"`
	}

	type R1 struct {
		ID         int64  `json:"id" comment:"уникальный номер"`
		Name       string `json:"name" comment:"Имя"`
		SubStruct  R4     `json:"siubstruct" comment:"ссылка"`
		SubStructs []R2   `json:"subarray" comment:"Массив доп.информация"`
	}

	r := NewResolver()

	r.Add("DELETE", "/two", func(w http.ResponseWriter, r *http.Request) { fmt.Print("two"); return }, "Comment for two", []interface{}{R1{}, R2{}, R3{}, R4{}}, []interface{}{})
	fmt.Printf("%s\n", string(r.makeJSONHelp()))
	fmt.Printf("%s\n", string(r.makeTxtHelp()))
}
