// Summer and Winter get their own look on the picker's tiles (home.vue) so
// the cashier can tell them apart at a glance: a patterned background (the
// sale-option-tile--<theme> classes there) and an icon. Any other option
// kashi adds gets a plain primary tile.
export function saleOptionLook(name: string): { theme: "summer" | "winter" | ""; icon: string } {
  switch (name.toLowerCase()) {
    case "summer":
      return { theme: "summer", icon: "mdi-weather-sunny" };
    case "winter":
      return { theme: "winter", icon: "mdi-weather-pouring" };
    default:
      return { theme: "", icon: "mdi-tag-outline" };
  }
}
