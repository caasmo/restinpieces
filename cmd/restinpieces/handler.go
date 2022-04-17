package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"math/rand"
	"strconv"
)

// all handlers should conform to fn(w http.ResponseWriter, r *http.Request)
//
// Differentiate from the Handler by using suffix

func (app *App) admin(w http.ResponseWriter, r *http.Request) {

	user := "testuser"
	//user := context.Get(r, "user")
	// Maybe other operations on the database
	json.NewEncoder(w).Encode(user)
}

func (app *App) tea(w http.ResponseWriter, r *http.Request) {
	//params := context.Get(r, "params").(httprouter.Params)
	//log.Println(params.ByName("id"))
	// tea := getTea(app.db, params.ByName("id"))
	json.NewEncoder(w).Encode(nil)
}

func (app *App) exampleSqliteReadRandom(w http.ResponseWriter, r *http.Request) {

	id := rand.Intn(100000) + 1
	value := app.db.GetById(int64(id))
	w.Write([]byte(`{"id":` + strconv.Itoa(id) + `,"value":` + strconv.Itoa(value) + `}`))
}

func (app *App) exampleWriteOne(w http.ResponseWriter, r *http.Request) {

	params := app.routerParam.Get(r.Context())
	valStr := params.ByName("value")
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		panic(err) // TODO
	}

	app.db.Insert(val)

	//value := app.db.Insert(r.Context())
	w.Write([]byte(`{"id":lipo` + `,"value":` + valStr + `}`))
}

func (app *App) benchmarkSqliteRWRatio(w http.ResponseWriter, r *http.Request) {

	params := app.routerParam.Get(r.Context())
	ratioStr := params.ByName("ratio")
	ratio, err := strconv.ParseInt(ratioStr, 10, 64)
	if err != nil {
		panic(err) // TODO
	}
	numReadsStr := params.ByName("reads")
	numReads, err := strconv.ParseInt(numReadsStr, 10, 64)
	if err != nil {

		panic(err) // TODO
	}

	// determine db call based on ratio
	nint := rand.Intn(100) + 1
	n64 := int64(nint)
	sum := 0
	var op string
	if n64 >= ratio {

		op = "write"
		//just use the ratio as value
		app.db.Insert(n64)
	} else {
		// how many reads
		op = "read"
		for i := 0; i < int(numReads); i++ {
			value := app.db.GetById(n64)
			sum = +value
		}
	}

	w.Write([]byte(`{"random num":` + strconv.Itoa(nint) + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op + `"}`))
}

func (app *App) benchmarkBaseline(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "Baseline")
}

func (app *App) benchmarkSqliteRWRatioPool(w http.ResponseWriter, r *http.Request) {

	params := app.routerParam.Get(r.Context())
	//fmt.Fprintf(os.Stderr, "[restinpieces] %v+\n", pams)
	ratioStr := params.ByName("ratio")

	ratio, err := strconv.ParseInt(ratioStr, 10, 64)
	if err != nil {
		panic(err) // TODO
	}
	numReadsStr := params.ByName("reads")
	numReads, err := strconv.ParseInt(numReadsStr, 10, 64)
	if err != nil {

		panic(err) // TODO
	}

	// determine db read or write based on ratio
	nint := rand.Intn(100) + 1
	n64 := int64(nint)
	sum := 0
	var op string
	if n64 >= ratio {

		op = "write"
		//just use the ratio as value
		app.db.InsertWithPool(n64)
	} else {
		// how many reads
		op = "read"
		for i := 0; i < int(numReads); i++ {
			value := app.db.GetById(n64)
			sum = +value
		}
	}

	w.Write([]byte(`{"random num":` + strconv.Itoa(nint) + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op + `"}`))
}

func (app *App) about(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You are on the about page.")
}

func (app *App) index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome!")
}
