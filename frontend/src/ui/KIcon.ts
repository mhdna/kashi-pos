import { defineComponent, h } from "vue";
import { iconPathFor } from "@/lib/icons";
import { resolveColor } from "./colors";
import { namedSizes, toUnit } from "./util";

// <k-icon icon="mdi-name" size="20" color="primary" /> (or the name as slot text).
export default defineComponent({
  name: "KIcon",
  props: {
    icon: String,
    size: [String, Number],
    color: String,
    start: Boolean,
    end: Boolean,
  },
  setup(props, { slots, attrs }) {
    return () => {
      let name = props.icon;
      if (!name && slots.default) {
        const text = slots.default()[0]?.children;
        if (typeof text === "string") name = text.trim();
      }
      const size = props.size === undefined ? undefined : String(props.size);
      const named = size !== undefined && namedSizes.includes(size);
      const style: Record<string, string> = {};
      if (size !== undefined && !named) style.fontSize = toUnit(size)!;
      const color = resolveColor(props.color);
      if (color) style.color = color;
      return h(
        "i",
        {
          class: [
            "k-icon",
            named && `k-icon--size-${size}`,
            props.start && "k-icon--start",
            props.end && "k-icon--end",
            attrs.onClick && "k-icon--clickable",
          ],
          style,
          "aria-hidden": "true",
        },
        [h("svg", { viewBox: "0 0 24 24", role: "img", "aria-hidden": "true" }, [h("path", { d: iconPathFor(name ?? "") })])],
      );
    };
  },
});
