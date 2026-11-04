import { useState } from "react";

export interface Todo {
  id: number;
  text: string;
  done: boolean;
}

export function TodoList({ todos, onToggle }: { todos: Todo[]; onToggle: (id: number) => void }) {
  const [filter, setFilter] = useState("all");

  function TodoRow({ todo }: { todo: Todo }) {
    return (
      <li style={{ textDecoration: todo.done ? "line-through" : "none" }} onClick={() => onToggle(todo.id)}>
        {todo.text}
      </li>
    );
  }

  const visible = todos.filter((t) => (filter === "done" ? t.done : true));
  return (
    <div style={{ padding: 8 }}>
      <select value={filter} onChange={(e) => setFilter(e.target.value)} className={["filter", filter].join(" ")}>
        <option value="all">All</option>
        <option value="done">Done</option>
      </select>
      <ul>
        {visible.map((todo) => (
          <TodoRow key={todo.id} todo={todo} />
        ))}
      </ul>
      <button onClick={() => setFilter("all")} data-extra={[filter]}>
        Reset
      </button>
    </div>
  );
}
