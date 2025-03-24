## Error response structure
    {
      "status": 400,
      "code": "invalid_input", // Machine-readable 
      "message": "The request contains invalid data.", // Human-readable explanation
      "data?": [ // optional details
        {
          "code": "max_length",        // Machine-readable issue type
          "message": "Password exceeds maximum length of 20 characters", // Human-readable explanation
          "param?": "password",          // The param causing the issue (optional if not field-specific)
          "value?": "mypasswordiswaytoolong123", // Optional: the problematic input
        },
        {
          "code": "required",
          "message": "Username is required"
          "param?": "username",
        }
      ]
    }

## Data response structure

- prefer flat. maybe data key. TODO
= trying to be mostly equssl toi error

    {
      "status": 200,
      "code": "invalid_input", // Machine-readable 
      "message": "The request contains invalid data.", // Human-readable explanation
      "data?": [ // always array
            { custom data structure

            



#### TODO

- details>message is the UI message shown. Can be dynamicaly created by server. 
- SDK gets the message and description to show in the UI.
- param can be used to locate position of the message ex in form validation



