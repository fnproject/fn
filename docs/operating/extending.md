# Building Custom Server with Extensions

To create an extension, there are just a couple of things to do.

* All config should be via ENV vars.

## Code

You need to register your extension with an init() method and write a function to be called 
for setup:

```go
func init() {
	server.RegisterExtension(&fnext.Extension{
		Name:  "logspam",
		Setup: setup, // Fn will call this during startup
	})
}

func setup(s *fnext.ExtServer) error {
    // Add all the hooks you extension needs here
	s.AddCallListener(&LogSpam{})
}
```
