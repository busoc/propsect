[.container[] | {
    name: .name,
    base: .baseContainer.name,
    space: ("/" + (.qualifiedName | split("/") | .[1:-1] | join("/"))),
    abstract: (.baseContainer.name == null),
    entries: [.entry[]? | {
      name:  (if .parameter.name then
        .parameter.qualifiedName
      else
        .container.name
      end),
      include: (.container.name != null)
    }]
  }]
