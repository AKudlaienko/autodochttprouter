package autodochttprouter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var (
	// Tab - отступ для каждого уровня в документации
	Tab = "  "
)

// IOField ...
type IOField struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Required bool       `json:"required"`
	Desc     string     `json:"description"`
	Link     []*IOField `json:"properties,omitempty"`
}

// IOStruct стуктура для хранения описания структур
type IOStruct struct {
	Name   string     `json:"name,omitempty"`
	Fields []*IOField `json:"fields,omitempty"`
}

// ComemntType структура для хранения комментария
type ComemntType struct {
	Method        string      `json:"method,omitempty"`
	Path          string      `json:"path,omitempty"`
	Desc          string      `json:"description,omitempty"`
	InputStructs  []*IOStruct `json:"input_structs,omitempty"`
	OutputStructs []*IOStruct `json:"output_structs,omitempty"`
}

// ResolvElement - элемент роутинга
type ResolvElement struct {
	F                     http.HandlerFunc
	RE                    *regexp.Regexp
	Method, Path, Comment string
	JSONHelp              []byte
}

// SortedKeysType тип для сортировки выводы
type SortedKeysType []string

func (a SortedKeysType) Len() int           { return len(a) }
func (a SortedKeysType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortedKeysType) Less(i, j int) bool { return a[i] < a[j] }

// ObjectCommentType - тип для описания структуры, которую надо показать в документации
type ObjectCommentType struct {
	Comment string
	Object  interface{}
}

// CitizenResolverType - собственный mux
type CitizenResolverType struct {
	handlers map[string]ResolvElement
}

// NewResolver создание нового резолвера
func NewResolver() CitizenResolverType {
	r := CitizenResolverType{handlers: make(map[string]ResolvElement)}
	r.Add("GET", "/mgmt/help", r.apiHelpFunc, "Help text", nil, nil)
	r.Add("GET", "/mgmt/help/txt", r.apiHelpFunc, "Help text", nil, nil)
	r.Add("GET", "/mgmt/help/json", r.apiHelpFuncJSON, "Help text", nil, nil)
	return r
}

// apiHelpFunc
func (r CitizenResolverType) apiHelpFunc(resp http.ResponseWriter, req *http.Request) {

	resp.Write(r.makeTxtHelp())
}

func (r CitizenResolverType) makeTxtHelp() []byte {
	var (
		sortedKeys SortedKeysType
	)

	type EndPoint struct {
		Method, Path, Comment string
	}

	ret := ""

	for k := range r.handlers {
		sortedKeys = append(sortedKeys, k)
	}

	sort.Sort(sortedKeys)

	for _, k := range sortedKeys {
		k = swapKey(k)
		h := r.handlers[k]
		if h.Comment != "-" {
			c := ""
			for i, s := range strings.Split(h.Comment, "\n") {
				if i == 0 {
					c = c + fmt.Sprintf("%s\n", s)
				} else {
					c = c + fmt.Sprintf("%s%s\n", Tab, s)
				}
			}

			ret = ret + fmt.Sprintf("Вызов: %s %s\n%s%s\n", h.Method, h.Path, Tab, c)
		}
	}

	return []byte(ret)
}

// apiHelpFuncJSON
func (r CitizenResolverType) apiHelpFuncJSON(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(r.makeJSONHelp())
}

func (r CitizenResolverType) makeJSONHelp() []byte {
	var (
		sortedKeys SortedKeysType
		// ret        []ComemntType
		js [][]byte
	)

	type EndPoint struct {
		Method, Path, Comment string
	}

	for k := range r.handlers {
		sortedKeys = append(sortedKeys, k)
	}

	sort.Sort(sortedKeys)

	for _, k := range sortedKeys {
		if r.handlers[k].Comment != "-" {
			js = append(js, r.handlers[k].JSONHelp)
		}
	}
	return bytes.Join([][]byte{[]byte("["), bytes.Join(js, []byte(",")), []byte("]")}, []byte(""))
}

func swapKey(s string) string {
	return strings.Join(strings.Split(s, " "), " ")
}

// Add функция добавления адреса в сервисе
//	method: любой или *, если принимает по этому пути любые методы
//	path: строка, подходящая для регулярного выражения
func (r *CitizenResolverType) Add(method, path string, f http.HandlerFunc, comment string, in []interface{}, out []interface{}) error {
	var (
		re  *regexp.Regexp
		key string
		err error
		j   []byte
	)

	_, path, key = makeMask(method, path)
	if re, err = regexp.Compile(`^` + key + `$`); err != nil {
		return err
	}
	key = path + " " + method

	if _, ok := r.handlers[key]; ok {
		return errors.New("Can't add new path to resolver. Not unique")
	}

	// работаем с help-ом
	jsonComment := ComemntType{Desc: comment, Method: method, Path: path}
	if comment != "-" {
		if len(in) > 0 {
			comment = comment + "\n" + Tab + "Структуры запроса" + ":\n"
			for _, o := range in {
				comment = comment + getDopInfo(o, 2) + "\n"
			}
			// JSON
			for _, o := range in {
				s := getDopInfoForJSON(o)
				jsonComment.InputStructs = append(jsonComment.InputStructs, &s)
			}
			jsonComment.InputStructs = glueStructs(jsonComment.InputStructs)
		}
		if len(out) > 0 {
			comment = comment + "\n" + Tab + "Структуры ответа" + ":\n"
			for _, o := range out {
				comment = comment + getDopInfo(o, 2) + "\n"
			}
			// JSON
			for _, o := range out {
				s := getDopInfoForJSON(o)
				jsonComment.OutputStructs = append(jsonComment.OutputStructs, &s)
			}
			jsonComment.OutputStructs = glueStructs(jsonComment.OutputStructs)
		}
		j, _ = json.Marshal(jsonComment)
	}
	r.handlers[key] = ResolvElement{F: f, Method: method, Path: path, RE: re, Comment: comment, JSONHelp: j}

	return nil
}

func glueStructs(objects []*IOStruct) []*IOStruct {
	type Element struct {
		Obj     *IOStruct
		IsChild bool
	}
	var (
		ret []*IOStruct
		mm  map[string]*Element
	)
	mm = make(map[string]*Element)
	// Шаг 1 - создаем словарь объектов
	for _, oOuter := range objects {
		mm[oOuter.Name] = &Element{Obj: oOuter}
	}
	// Шаг 2 - бежим по объектам
	for _, oOuter := range objects {
		// В каждом объекте бежим по полям
		for _, f := range oOuter.Fields {
			if strings.Contains(f.Type, ".") {
				for k, o := range mm {
					if k != oOuter.Name && strings.Contains(f.Type, k) {
						// если нашли совпадение, то связываем поля
						o.IsChild = true
						f.Link = o.Obj.Fields
					}
				}
			}
		}
	}
	// Шаг 3 - прореживаем набор
	for _, o := range objects {
		if !mm[o.Name].IsChild {
			ret = append(ret, o)
		}
	}

	return ret
}

func getDopInfoForJSON(o interface{}) IOStruct {
	var (
		ret IOStruct
	)
	val := reflect.Indirect(reflect.ValueOf(o))
	t := val.Type()
	ret.Name = t.String()
	for i := 0; i < t.NumField(); i++ {
		f := getFieldCommentForJSON(t.Field(i))
		if f != nil {
			ret.Fields = append(ret.Fields, f)
		}
	}
	return ret
}

func getDopInfo(o interface{}, level int) string {
	var (
		ret string
		// err error
		tab string
	)

	for i := 0; i < level; i++ {
		tab = fmt.Sprintf("%s%s", tab, Tab)
	}

	val := reflect.Indirect(reflect.ValueOf(o))
	t := val.Type()
	ret = tab + "Структура " + t.String() + "\n"
	tab = tab + Tab
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := getFieldComment(f)
		if name != "" {
			ret = ret + fmt.Sprintf("%s%s\n", tab, name)
		}
	}
	return ret
}

func getFieldCommentForJSON(f reflect.StructField) *IOField {

	r := IOField{}

	tComment := f.Tag.Get("comment")
	if tComment == "-" {
		return nil
	}
	tJSON := strings.Replace(f.Tag.Get("json"), ",omitempty", "", 1)
	if tJSON == "-" {
		return nil
	}
	if tJSON != "" {
		r.Name = tJSON
	} else {
		r.Name = f.Name
	}
	r.Type = fmt.Sprintf("%s", f.Type)

	if tComment != "" {
		r.Desc = tComment
	}

	return &r
}

func getFieldComment(f reflect.StructField) string {
	r := ""
	tComment := f.Tag.Get("comment")
	if tComment == "-" {
		return ""
	}
	tJSON := strings.Replace(f.Tag.Get("json"), ",omitempty", "", 1)
	if tJSON == "-" {
		return ""
	}

	if tJSON != "" {
		r = tJSON
	} else {
		r = f.Name
	}
	r = r + "(" + fmt.Sprintf("%s", f.Type) + ")"

	if tComment != "" {
		r = r + ": " + tComment
	}

	return r
}

func makeMask(method, path string) (string, string, string) {
	method = strings.ToUpper(method)
	path = strings.TrimRight(path, "/")
	if method == "*" {
		method = `\w+`
	}
	key := method + " " + path

	path = strings.Replace(path, `(\d+)`, `[0-9]`, -1)
	path = strings.Replace(path, `(\w+)`, `[a-zA-Z]`, -1)

	return method, path, key
}

// Match проверка совпадения маски и адреса
func (r CitizenResolverType) Match(key string) (ResolvElement, bool, []string) {
	var retPar []string
	for _, v := range r.handlers {
		if v.RE.Match([]byte(key)) {
			for i, b := range v.RE.FindSubmatch([]byte(key)) {
				if i != 0 {
					retPar = append(retPar, string(b))
				}
			}
			return v, true, retPar
		}
	}
	return ResolvElement{}, false, []string{}
}

// ServeHTTP функция добавления адреса в сервисе
func (r *CitizenResolverType) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	_, _, key := makeMask(req.Method, req.URL.Path)

	h, ok, _ := r.Match(key)
	if !ok {
		http.NotFound(res, req)
		return
	}
	// TODO: сделать их req http.NewRequest, куда добавить полученные параметры
	h.F(res, req)
}
