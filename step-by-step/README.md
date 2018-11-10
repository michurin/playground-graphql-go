# Simple step-by-step examples

Accompanying materials for my talk on [DevFest Siberia 2018](https://gdg-siberia.com/).

## minimal.go

- Minimal example: one key and on constant response as string "b"

```
query {a}
```

## args.go

- Use integer response
- Two keys
- Use one integer argument
- List in response

```
query {a} # Error: we need nonnull argument
query {a(x:2)}
query {a(x:2) b(x:5)}
query {a b} # Collect all errors
query {a(x:2) b(x:30)} # Error but collert all reachable data
```

## type.go

- Simplest custom type with Resolver interface
- Go structures not must to corelate with GraphQL structs
- Resolver resolves each field individualy
- You init part of object in parent resolver

```
query {a} # You can't call all attrs
query {a {nm}}
query {a {nnm}} # Try to suggest fied names
```

## recursion.go

- Add field by `AddFieldConfig`

```
query {a {nm sub{nm}}}
query {a {nm sub{nm sub{nm sub{nm sub{nm}}}}}}
```

## defer.go

- return `func() (interface{}, error)`
- use async resolver

```
query {a {nm}}
```

## processing.go

- Order of processing

```
query {a{seq sub{seq sub{seq sub{seq}}}}}
```
