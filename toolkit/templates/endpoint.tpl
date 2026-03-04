## Endpoint: {{ .Name }}

### Success: {{ .Passed }}

**Method:** {{ .Method }}
**Test ID** `{{ .TestID }}`

**Expected Outputs:** {{ .ExpectedStatus }}
**Booted Code:** {{ .Status }}

**Booted Output:**
{{ .Body }}