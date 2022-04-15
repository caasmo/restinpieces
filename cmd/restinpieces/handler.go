package main

import (
    "encoding/json"
    "net/http"
    "fmt"
    //"os"
    "math/rand"
    "strconv"
    "github.com/julienschmidt/httprouter"
)

// all handlers should conform to fn(w http.ResponseWriter, r *http.Request) 
//
// Differentiate from the Handler by using suffix 

func (c *App) admin(w http.ResponseWriter, r *http.Request) {

    user := "testuser"
    //user := context.Get(r, "user")
    // Maybe other operations on the database
    json.NewEncoder(w).Encode(user)
}


func (c *App) tea(w http.ResponseWriter, r *http.Request) {
    //params := context.Get(r, "params").(httprouter.Params)
    //log.Println(params.ByName("id"))
    // tea := getTea(c.db, params.ByName("id"))
    json.NewEncoder(w).Encode(nil)
}

func (c *App) exampleSqliteReadRandom(w http.ResponseWriter, r *http.Request) {

    id := rand.Intn(100000)+1
    value := c.dbase.GetById(int64(id))
	w.Write([]byte(`{"id":` + strconv.Itoa(id) + `,"value":` + strconv.Itoa(value) + `}`))
}

func (c *App) exampleWriteOne(w http.ResponseWriter, r *http.Request) {

    params := httprouter.ParamsFromContext(r.Context())
	//fmt.Fprintf(os.Stderr, "[restinpieces] %v\n", params)
    valStr := params.ByName("value")
    val, err := strconv.ParseInt(valStr, 10, 64);
    if err != nil {
        panic(err) // TODO
    }

    c.dbase.Insert(val)

    //value := c.dbase.Insert(r.Context())
	w.Write([]byte(`{"id":lipo` + `,"value":` + valStr+ `}`))
}

func (c *App) benchmarkSqliteRWRatio(w http.ResponseWriter, r *http.Request) {

    params := httprouter.ParamsFromContext(r.Context())
	//fmt.Fprintf(os.Stderr, "[restinpieces] %v\n", params)
    ratioStr := params.ByName("ratio")
    ratio, err := strconv.ParseInt(ratioStr, 10, 64); 
    if err != nil {
        panic(err) // TODO
    }
    numReadsStr := params.ByName("reads")
    numReads, err := strconv.ParseInt(numReadsStr, 10, 64) 
    if err != nil {

        panic(err) // TODO
    }

    // determine db call based on ratio
    nint := rand.Intn(100)+1
    n64 := int64(nint)
    sum := 0
    var op string
    if n64 >= ratio {

        op = "write"
        //just use the ratio as value
        c.dbase.Insert(n64)
    } else {
        // how many reads
        op = "read"
        for i := 0; i< int(numReads); i++ {
            value := c.dbase.GetById(n64)
            sum=+value
        }
    }


	w.Write([]byte(`{"random num":` + strconv.Itoa(nint)  + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op +`"}`))
}

func (c *App) benchmarkSqliteRWRatioPool(w http.ResponseWriter, r *http.Request) {

    params := httprouter.ParamsFromContext(r.Context())
	//fmt.Fprintf(os.Stderr, "[restinpieces] %v\n", params)
    ratioStr := params.ByName("ratio")
    ratio, err := strconv.ParseInt(ratioStr, 10, 64); 
    if err != nil {
        panic(err) // TODO
    }
    numReadsStr := params.ByName("reads")
    numReads, err := strconv.ParseInt(numReadsStr, 10, 64) 
    if err != nil {

        panic(err) // TODO
    }

    // determine db call based on ratio
    nint := rand.Intn(100)+1
    n64 := int64(nint)
    sum := 0
    var op string
    if n64 >= ratio {

        op = "write"
        //just use the ratio as value
        c.dbase.InsertWithPool(n64)
    } else {
        // how many reads
        op = "read"
        for i := 0; i< int(numReads); i++ {
            value := c.dbase.GetById(n64)
            sum=+value
        }
    }


	w.Write([]byte(`{"random num":` + strconv.Itoa(nint)  + `,"sum":` + strconv.Itoa(sum) + `,"operation":"` + op +`"}`))
}


func (c *App) about(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "You are on the about page.")
}

func (c *App) index(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome!")
}

