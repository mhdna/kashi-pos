// French for the till: English text -> French. Each file in ./fr holds one area
// (default export: { "English": "Français" }); {placeholders} must stay as they are.
const files = import.meta.glob<{ default: Record<string, string> }>("./fr/*.ts", { eager: true });

export const FR: Record<string, string> = Object.assign({}, ...Object.values(files).map((m) => m.default));
