# 

    hey -z 10s  -c 100 -q 50 "http://localhost:8080"

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

