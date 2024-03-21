# traefik-request-filter

## WIP

### Configuration

The following declaration (given here in YAML) defines a plugin:

```
# Static configuration

experimental:
  plugins:
    traefik-request-filter:
      moduleName: github.com/0xanonymeow/traefik-request-filter
      version: v0.1.0

```

Here is an example of a file provider dynamic configuration (given here in YAML), where the interesting part is the http.middlewares section:

```
# Dynamic configuration

http:
  routers:
    my-router:
      rule: host(`demo.localhost`)
      service: service-foo
      entryPoints:
        - web
      middlewares:
        - requestFilter@docker

  services:
   service-foo:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:80

  middlewares:
    requestFilter:
      plugin:
        traefik-request-filter:
          headers:
            X-Foo: bar
            X-Bar: baz,qux;quux
          query:
            foo: bar
            bar: baz,qux;quux
          body:
            # regex: ^foo$
            json:
              foo: bar
              bar: true
              baz: 100
              qux:
                - quux
                - true
                - 100
```