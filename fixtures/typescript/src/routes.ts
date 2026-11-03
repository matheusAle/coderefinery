import { Router } from "./router";

const users = [
  { id: "1", name: "ann", admin: true },
  { id: "2", name: "bob", admin: false },
];

export const router = new Router();

router.get("/users", (req, res) => {
  const onlyAdmins = req.query.admin === "true";
  res.json(onlyAdmins ? users.filter((u) => u.admin) : users);
});

router.get("/users", (req, res) => {
  if (req.query.limit) {
    res.status(206);
  }
});

router.post("/users", (req, res) => {
  const name = req.body?.name;
  if (typeof name !== "string" || name.length === 0) {
    res.status(400).json({ error: "name required" });
    return;
  }
  res.status(201).json({ id: String(users.length + 1), name });
});

router.delete("/users/:id", (req, res) => {
  const index = users.findIndex((u) => u.id === req.params.id);
  if (index < 0 && !req.query.force) {
    res.status(404).json({ error: "missing" });
    return;
  }
  res.status(204).json(null);
});

export function routeCount(): number {
  return router.routes.length;
}
