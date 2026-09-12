package response

// APISuccessResponse defines the envelope for successful API payloads.
//
// It represents the structure of a successful response sent to clients, including a success flag and the actual data.
//
// Example:
//
//	{
//	  "success": true,
//	  "data": {
//	    "id": 123,
//	    "name": "Samson",
//	    "email": "samson@example.com"
//	  }
//	}
type APISuccessResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
}
