# Function timeouts

Within Function API, each functions supplied with 2 timeouts parameters, both optional, see [swagger.yaml](swagger.yml) for more details.
So, what are those timeouts and what are they used for?

## Function call timeout

This time of timeouts defines for how long function execution may happen before it'll be terminated along with notifying caller that function terminated with error - timed out.

```json
{
	"route":{
	    ...
		"timeout": 30,
	    ...
	}
}
```

This timeout parameter used with both types of functions: async and sync.
It starts at the beginning of function call.

## Hot function idle timeout

This type of timeout defines for how long should hot function hang around before its termination in case if there are no incoming requests.

```json
{
	"route":{
	    ...
		"idle_timeout": 30,
	    ...
	}
}
```

This timeout parameter is valid for hot functions, see what [hot functions](hot-functions.md) is. By default this parameter equals to 30 seconds.
It starts after last request being processed by hot function.

## Correlation between idle and regular timeout

This two timeouts are independent. The order of timeouts for hot functions:

 0. start hot function be sending first timeout-bound request to it
 1. make request to function with `timeout`
 2. if call finished (no matter successful or not) check for more requests to dispatch
 3. if none - start idle timeout
 4. if new request appears - stop idle timeout and serve request
 5. if none - terminate hot function

## Hot function idle timeout edge cases

Having both timeouts may cause confusion while configuring hot function.
So, there are certain limitations for `idle_timeout` as well as for regular `timeout`:

 * Idle timeout might be equal to zero. Such case may lead to satiation when function would be terminated immediately after last request processing, i.e. no idle timeout at all.
 * Idle timeout can't be negative.
 * Idle timeout can't be changed while hot function is running. Idle timeout is permanent within hot function execution lifecycle. It means that idle timeout should be considered for changing once functions is not running.
