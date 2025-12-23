# choria

Choria will allow use to do fleet management of Servers and desktops.

It wil then also allow use to do fleet management of drones in terms of what is deplyoed on them.

It uses NATS Jetstream, and has extensive opportunities.

This is just another sub system for us.

There are many repositories in choria, and many of them are very powerful when used togehter in the right way.


It has its own Taskfile system called ABTaskFile.


## notes

https://github.com/choria-io/

https://github.com/orgs/choria-io/repositories

---

Backplane Development Framework and Server hosting Choria Agents, Networks, Federations and Streaming Data

https://github.com/choria-io/go-choria

docs: https://choria-io.github.io/go-choria/

users:

https://github.com/holos-run/holos looks interesting.


--

Choria Configuration Management

https://github.com/choria-io/ccm

docs: https://choria-io.github.io/ccm/s



---

Tool to create friendly wrapping command lines over operations tools

https://github.com/choria-io/appbuilder

doc: https://choria-io.github.io/appbuilder/

---

Schemas for various Choria projects

https://github.com/choria-io/schemas

---

https://github.com/choria-io/website/

We are in luck, as it uses Hugo, just like we do.

This is a perfect example and starter for us to start to get docs system for each of our sub systems. 

Each Sub system will have a docs folder.

The current docs foler in the root can change to being called hugo, because that is what it is. 

docs folder has some Architetcure stuff in it. 

I guess we have to work out if we copy the sub system docs into it or if we can just have the hugo docs sub system reference the docs in the other sub systems. I prefer a reference approach, instead of copying.

Also Choria is external docs, and we have intewrnal docs too, so we can start this off and get our patterns in order.


---

Data replication for NATS JetStream

https://github.com/choria-io/stream-replicator

docs: https://choria-io.github.io/stream-replicator/

I can see use needing this. 

Need to see why this exists actually, because NATS sort of does this ? 