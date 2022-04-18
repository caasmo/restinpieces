package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"math/rand"
	"os"
	"time"
	"strconv"
)

// all handlers should conform to fn(w http.ResponseWriter, r *http.Request)
//
// Differentiate from the Handler by using suffix

func (a *App) Admin(w http.ResponseWriter, r *http.Request) {

	user := "testuser"
	//user := context.Get(r, "user")
	// Maybe other operations on the database
	json.NewEncoder(w).Encode(user)
}

func (a *App) Tea(w http.ResponseWriter, r *http.Request) {
	//params := context.Get(r, "params").(httprouter.Params)
	//log.Println(params.ByName("id"))
	// tea := getTea(a.db, params.ByName("id"))
	json.NewEncoder(w).Encode(nil)
}

func (a *App) ExampleSqliteReadRandom(w http.ResponseWriter, r *http.Request) {

	id := rand.Intn(100000) + 1
	value := a.db.GetById(int64(id))
	w.Write([]byte(`{"id":` + strconv.Itoa(id) + `,"value":` + strconv.Itoa(value) + `}`))
}

func (a *App) ExampleWriteOne(w http.ResponseWriter, r *http.Request) {

	params := a.routerParam.Get(r.Context())
	valStr := params.ByName("value")
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		panic(err) // TODO
	}

	a.db.Insert(val)

	//value := a.db.Insert(r.Context())
	w.Write([]byte(`{"id":lipo` + `,"value":` + valStr + `}`))
}

func (a *App) BenchmarkSqliteRWRatio(w http.ResponseWriter, r *http.Request) {

	params := a.routerParam.Get(r.Context())
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
		a.db.Insert(n64)
	} else {
		// how many reads
		op = "read"
		for i := 0; i < int(numReads); i++ {
			value := a.db.GetById(n64)
			sum = +value
		}
	}

	w.Write([]byte(`{"random num":` + strconv.Itoa(nint) + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op + `"}`))
}

func (a *App) BenchmarkBaseline(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "Baseline")
}


func (a *App) BenchmarkRistrettoRead() http.HandlerFunc {
    // set one time 
    b := a.cache.Set("hi", "hola", 1)
	fmt.Fprintf(os.Stderr, "[restinpieces] set hi key in cache ristretto %v+\n", b)

    time.Sleep(10 * time.Millisecond)

    return func(w http.ResponseWriter, r *http.Request) {
        value, found := a.cache.Get("hi")

        if !found {
		    http.Error(w, http.StatusText(401), 401)
		    return
        }

        v, _ := value.(string)

	    w.Write([]byte(`{"Value from ristretto cache hi": "` + v  + `"}`))
    }
}

func (a *App) BenchmarkSqliteRWRatioPool(w http.ResponseWriter, r *http.Request) {

	params := a.routerParam.Get(r.Context())
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
		a.db.InsertWithPool(n64)
	} else {
		// how many reads
		op = "read"
		for i := 0; i < int(numReads); i++ {
			value := a.db.GetById(n64)
			sum = +value
		}
	}

	w.Write([]byte(`{"random num":` + strconv.Itoa(nint) + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op + `"}`))
}

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome!")
}
