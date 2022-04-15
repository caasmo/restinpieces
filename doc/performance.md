# 3fd3986 sqlite read and writes with ratio, no logging, 2 reads when reading, 1 write when writing

    // 80% reads 20% writes , 2 pk reads per request
    hey -n 100000  "http://localhost:8080/benchmark/sqlite/ratio/80/read/2"
    Requests/sec: 40408.4698

    // 100% writes 1 pk write per request 
    hey -n 100000  "http://localhost:8080/benchmark/sqlite/ratio/0/read/0" 
    Requests/sec: 17003.2902

    // 100% reads 1 pk read per request 
    hey -n 100000  "http://localhost:8080/benchmark/sqlite/ratio/100/read/1" 
    Requests/sec: 64157.3485

# bcea0ea, select by random random to table of 100000 sqlite

    hey -n 100000  "http://localhost:8080/db"

    Summary:
      Total:        2.4530 secs
      Slowest:      0.0140 secs
      Fastest:      0.0001 secs
      Average:      0.0012 secs
      Requests/sec: 40766.0430

      Total data:   2278877 bytes
      Size/request: 22 bytes

    Response time histogram:
      0.000 [1]     |
      0.001 [73747] |■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
      0.003 [22377] |■■■■■■■■■■■■
      0.004 [3031]  |■■
      0.006 [620]   |
      0.007 [173]   |
      0.008 [33]    |
      0.010 [9]     |
      0.011 [5]     |
      0.013 [1]     |
      0.014 [3]     |

    Status code distribution:
      [200] 100000 responses

# aae2c5b, just barebones routing with empty handlers and basic logging in the console.

     hey -z 10s  -c 100 -q 50 "http://localhost:8080"

    Summary:
      Total:        10.0081 secs
      Slowest:      0.0227 secs
      Fastest:      0.0001 secs
      Average:      0.0014 secs
      Requests/sec: 4995.9660

      Total data:   400000 bytes
      Size/request: 8 bytes

    Response time histogram:
      0.000 [1]     |
      0.002 [46423] |■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
      0.005 [3474]  |■■■
      0.007 [56]    |
      0.009 [0]     |
      0.011 [0]     |
      0.014 [0]     |
      0.016 [0]     |
      0.018 [0]     |
      0.020 [0]     |
      0.023 [46]    |

    Status code distribution:
    [200] 50000 responses
