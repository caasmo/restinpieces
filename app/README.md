## Error response structure
    {
      "status": 400,
      "code": "invalid_input", // Machine-readable 
      "message": "The request contains invalid data.", // Human-readable explanation
      "details": [ // optional
        {
          "issue": "max_length",        // Machine-readable issue type
          "description": "Password exceeds maximum length of 20 characters", // Human-readable explanation
          "param": "password",          // The param causing the issue (optional if not field-specific)
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

- not convinced of constraint, you can not build a programatically user facing message with this info => Excluded
- description is the UI message shown. Can be dynamicaly created by server. 
- SDK gets the message and description to show in the UI.


