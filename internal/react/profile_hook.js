// Render profiler for `coderefinery react --profile` (docs/design-react.md §10).
// Installed as the React DevTools global hook before react-dom loads: React
// calls onCommitFiberRoot after every commit and this walks the committed
// fibers the way React DevTools' Profiler does. Per component name it counts
// renders (commits in which the component's function or render() ran),
// mounts, and sums React's actualDuration (ms, the fiber and the descendants
// that rendered with it). The test runner calls __coderefineryFlush() after
// each test file; it appends the counts to a JSON file in OUT.
var KEY = "__REACT_DEVTOOLS_GLOBAL_HOOK__";
var PERFORMED_WORK = 1;
// FunctionComponent, ClassComponent, ForwardRef, SimpleMemoComponent. A
// MemoComponent (14) is skipped: its inner fiber renders and is counted.
var COMPONENT_TAGS = { 0: true, 1: true, 11: true, 15: true };

function profilerHook() {
  var existing = globalThis[KEY];
  if (existing && existing.__coderefinery) return existing;
  var hook = {
    __coderefinery: true,
    supportsFiber: true,
    renderers: new Map(),
    store: { renderers: [], components: {}, untimed: false, error: "" },
    inject: function (internals) {
      var id = hook.renderers.size + 1;
      hook.renderers.set(id, internals);
      hook.store.renderers.push({ package: String(internals.rendererPackageName || ""),
        version: String(internals.version || ""), bundleType: internals.bundleType });
      return id;
    },
    onCommitFiberRoot: function (id, root) {
      try {
        commitRoot(hook.store, root);
      } catch (err) {
        hook.store.error = String(err && err.stack || err);
      }
    },
    onCommitFiberUnmount: function () {},
    onPostCommitFiberRoot: function () {},
    setStrictMode: function () {},
  };
  globalThis[KEY] = hook;
  return hook;
}

function flagsOf(fiber) {
  return fiber.flags !== undefined ? fiber.flags : fiber.effectTag;
}

function nameOf(fiber) {
  var type = fiber.type, element = fiber.elementType;
  if (!type) return "";
  if (fiber.tag === 11) {
    var render = type.render;
    return type.displayName || (render && (render.displayName || render.name)) || "";
  }
  if (fiber.tag === 15 && element && element.displayName) return element.displayName;
  return type.displayName || type.name || "";
}

function record(store, fiber, mount) {
  if (!COMPONENT_TAGS[fiber.tag] || (flagsOf(fiber) & PERFORMED_WORK) === 0) return;
  var name = nameOf(fiber);
  if (!name) return;
  var c = store.components[name];
  if (!c) c = store.components[name] = { renders: 0, mounts: 0, totalMs: 0, maxMs: 0 };
  c.renders++;
  if (mount) c.mounts++;
  var d = fiber.actualDuration;
  if (typeof d !== "number") {
    store.untimed = true;
    return;
  }
  c.totalMs += d;
  if (d > c.maxMs) c.maxMs = d;
}

function mountTree(store, fiber) {
  for (var f = fiber; f; f = f.sibling) {
    record(store, f, true);
    mountTree(store, f.child);
  }
}

// updateTree visits a fiber kept from the previous commit; a subtree whose
// child list was not re-created did not render and is skipped.
function updateTree(store, next, prev) {
  record(store, next, false);
  if (next.child === prev.child) return;
  for (var c = next.child; c; c = c.sibling) {
    if (c.alternate) updateTree(store, c, c.alternate);
    else mountTree(store, c);
  }
}

function hasElement(fiber) {
  return fiber != null && fiber.memoizedState != null && fiber.memoizedState.element != null;
}

function commitRoot(store, root) {
  var current = root.current, previous = current.alternate;
  if (!hasElement(current)) return;
  if (hasElement(previous)) updateTree(store, current, previous);
  else mountTree(store, current.child);
}

profilerHook();

function __coderefineryFlush() {
  var hook = globalThis[KEY];
  if (!hook || !hook.__coderefinery) return;
  var store = hook.store;
  if (store.renderers.length === 0 && Object.keys(store.components).length === 0 && !store.error) return;
  var name = "profile-" + process.pid + "-" + Date.now() + "-" + Math.random().toString(36).slice(2) + ".json";
  writeFile(OUT + "/" + name, JSON.stringify(store));
  hook.store = { renderers: [], components: {}, untimed: false, error: "" };
}
