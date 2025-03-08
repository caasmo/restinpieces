## Error response structure
    {
      "status": 400,
      "code": "invalid_input", // Machine-readable 
      "message": "The request contains invalid data.", // Human-readable explanation
      "details": [ // optional
        {
          "param": "password",          // The param causing the issue (optional if not field-specific)
          "issue": "max_length",        // Machine-readable issue type
          "description": "Password exceeds maximum length of 20 characters", // Human-readable explanation
          "value": "mypasswordiswaytoolong123", // Optional: the problematic input
          "constraint": {               // Excluded: specific constraint details 
            "max_length": 20
          }
        },
        {
          "param": "username",
          "issue": "required",
          "description": "Username is required"
        }
      ]
    }


#### TODO

- not convinced of constraint, you can not build a programatically user facing message with this info
- INstead Client SDK should have a precomputed list of [code, issue] => "harcoded human message"
- the hardcoded human message can be dinamically made by the server "%s exceeds
  maximum length of 20 characters" with field1, field2 rendering in the server


