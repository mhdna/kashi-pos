// Summer and Winter get their own colour and icon so the cashier can tell
// them apart at a glance -- on the picker's tiles (home.vue) and on the cart
// line's chip (InvoiceItems.vue). Any other option kashi adds uses the
// primary look.
export function saleOptionLook(name: string): { color: string; icon: string } {
  switch (name.toLowerCase()) {
    case "summer":
      return { color: "#FF7A00", icon: "mdi-weather-sunny" };
    case "winter":
      return { color: "#1E90FF", icon: "mdi-weather-pouring" };
    default:
      return { color: "primary", icon: "mdi-tag-outline" };
  }
}
