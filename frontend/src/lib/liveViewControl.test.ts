// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { applyControl, parseControl, type NodeMirror } from "./liveViewControl";

describe("parseControl", () => {
  it("reads the three messages kashi sends", () => {
    expect(parseControl('{"t":"click","id":7}')).toEqual({ t: "click", id: 7 });
    expect(parseControl('{"t":"input","id":7,"v":"abc"}')).toEqual({ t: "input", id: 7, v: "abc" });
    expect(parseControl('{"t":"key","key":"Enter"}')).toEqual({ t: "key", key: "Enter" });
  });
  it("refuses everything else", () => {
    for (const bad of [
      "", "nope", "null", "5", "{}", '{"t":"eval","code":"x"}', '{"t":"snapshot"}',
      '{"t":"click"}', '{"t":"click","id":0}', '{"t":"click","id":-3}', '{"t":"click","id":1.5}', '{"t":"click","id":"7"}',
      '{"t":"input","id":7}', '{"t":"input","id":7,"v":5}', '{"t":"input","id":0,"v":"x"}',
      '{"t":"key"}', '{"t":"key","key":"F5"}', '{"t":"key","key":"Control"}', '{"t":"key","key":""}', '{"t":"key","key":"ab"}',
    ]) {
      expect(parseControl(bad), bad).toBeNull();
    }
  });
  it("bounds a pasted value", () => {
    const m = parseControl(JSON.stringify({ t: "input", id: 1, v: "x".repeat(5000) }));
    expect(m && m.t === "input" && m.v.length).toBe(2000);
  });
});

function setup(html: string) {
  document.body.innerHTML = html;
  const nodes = new Map<number, Node>();
  let next = 1;
  const id = (node: Node) => {
    nodes.set(next, node);
    return next++;
  };
  const mirror: NodeMirror = { getNode: (n) => nodes.get(n) ?? null };
  return { id, mirror };
}

describe("applyControl", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("a click clicks the element and focuses it", () => {
    const { id, mirror } = setup('<button id="b">Pay</button><input id="i">');
    const button = document.getElementById("b")!;
    const onClick = vi.fn();
    button.addEventListener("click", onClick);
    expect(applyControl({ t: "click", id: id(button) }, mirror)).toBe(true);
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(button);
  });

  it("a click on a text node clicks its parent; an unknown or detached node does nothing", () => {
    const { id, mirror } = setup('<button id="b">Pay</button>');
    const button = document.getElementById("b")!;
    const onClick = vi.fn();
    button.addEventListener("click", onClick);
    expect(applyControl({ t: "click", id: id(button.firstChild!) }, mirror)).toBe(true);
    expect(onClick).toHaveBeenCalledTimes(1);

    expect(applyControl({ t: "click", id: 999 }, mirror)).toBe(false);
    const detached = id(button);
    button.remove();
    expect(applyControl({ t: "click", id: detached }, mirror)).toBe(false);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("input sets the value the way typing does: an input and a change event", () => {
    const { id, mirror } = setup('<input id="i">');
    const input = document.getElementById("i") as HTMLInputElement;
    const seen: string[] = [];
    input.addEventListener("input", () => seen.push("input:" + input.value));
    input.addEventListener("change", () => seen.push("change:" + input.value));
    expect(applyControl({ t: "input", id: id(input), v: "hello" }, mirror)).toBe(true);
    expect(input.value).toBe("hello");
    expect(seen).toEqual(["input:hello", "change:hello"]);
  });

  it("input on something that is not a text field does nothing", () => {
    const { id, mirror } = setup('<input id="c" type="checkbox"><div id="d"></div>');
    expect(applyControl({ t: "input", id: id(document.getElementById("c")!), v: "x" }, mirror)).toBe(false);
    expect(applyControl({ t: "input", id: id(document.getElementById("d")!), v: "x" }, mirror)).toBe(false);
  });

  it("keys type into the focused field at the caret and Backspace removes", () => {
    const { id, mirror } = setup('<input id="i">');
    const input = document.getElementById("i") as HTMLInputElement;
    applyControl({ t: "click", id: id(input) }, mirror);
    for (const key of ["1", "2", "3"]) applyControl({ t: "key", key }, mirror);
    expect(input.value).toBe("123");
    input.setSelectionRange(1, 1);
    applyControl({ t: "key", key: "9" }, mirror);
    expect(input.value).toBe("1923");
    applyControl({ t: "key", key: "Backspace" }, mirror);
    expect(input.value).toBe("123");
    applyControl({ t: "key", key: "Delete" }, mirror);
    expect(input.value).toBe("13");
  });

  it("a key the page cancels is not typed", () => {
    const { id, mirror } = setup('<input id="i">');
    const input = document.getElementById("i") as HTMLInputElement;
    input.addEventListener("keydown", (e) => e.preventDefault());
    applyControl({ t: "click", id: id(input) }, mirror);
    applyControl({ t: "key", key: "a" }, mirror);
    expect(input.value).toBe("");
  });

  it("Enter reaches keydown handlers and presses a focused button", () => {
    const { id, mirror } = setup('<input id="i"><button id="b">Go</button>');
    const input = document.getElementById("i")!;
    const button = document.getElementById("b")!;
    const keys: string[] = [];
    input.addEventListener("keydown", (e) => keys.push((e as KeyboardEvent).key));
    applyControl({ t: "click", id: id(input) }, mirror);
    applyControl({ t: "key", key: "Enter" }, mirror);
    expect(keys).toEqual(["Enter"]);

    const onClick = vi.fn();
    button.addEventListener("click", onClick);
    button.focus();
    applyControl({ t: "key", key: "Enter" }, mirror);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("Tab moves focus to the next field and wraps", () => {
    const { id, mirror } = setup('<input id="a"><input id="b">');
    const a = document.getElementById("a")!;
    const b = document.getElementById("b")!;
    applyControl({ t: "click", id: id(a) }, mirror);
    applyControl({ t: "key", key: "Tab" }, mirror);
    expect(document.activeElement).toBe(b);
    applyControl({ t: "key", key: "Tab" }, mirror);
    expect(document.activeElement).toBe(a);
  });

  it("a key with nothing focused still reaches the page", () => {
    const { mirror } = setup("<div></div>");
    const seen: string[] = [];
    document.body.addEventListener("keydown", (e) => seen.push(e.key));
    expect(applyControl({ t: "key", key: "Escape" }, mirror)).toBe(true);
    expect(seen).toEqual(["Escape"]);
  });

  // Whatever mix of typing and Backspace a watcher does, the field ends up
  // exactly as a model of a text box says.
  it("randomised typing matches a plain string model", () => {
    for (let seed = 1; seed <= 200; seed++) {
      let state = seed;
      const rand = (n: number) => {
        state = (state * 1103515245 + 12345) & 0x7fffffff;
        return state % n;
      };
      const { id, mirror } = setup('<input id="i">');
      const input = document.getElementById("i") as HTMLInputElement;
      applyControl({ t: "click", id: id(input) }, mirror);
      let model = "";
      for (let step = 0; step < 40; step++) {
        const roll = rand(10);
        if (roll < 6) {
          const ch = "abc123 €"[rand(8)]!;
          applyControl({ t: "key", key: ch }, mirror);
          model += ch;
        } else if (roll < 8) {
          applyControl({ t: "key", key: "Backspace" }, mirror);
          model = model.slice(0, -1);
        } else {
          const v = "paste" + rand(100);
          applyControl({ t: "input", id: 1, v }, mirror);
          model = v;
        }
        input.setSelectionRange(model.length, model.length);
        expect(input.value, `seed ${seed} step ${step}`).toBe(model);
      }
    }
  });
});
