# Running on SELinux systems

Systems such as OEL 7.x where SELinux is enabled and the security policies are set to "Enforcing" will restrict Fn from
running containers and mounting volumes.

For local development, you can relax SELinux constraints by running this command in a root shell:

```sh
setenforce permissive
```

Then you will be able to run `fn start` as normal.

Of course this isn't recommended for operating Fn in production systems, for security reasons. Check the operating
[options](docs/operating/options.md) instead.
