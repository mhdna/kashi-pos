// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ui from "@/ui";
import golden from "@/lib/testdata/invoice_line_details.json";
import InvoiceLineDetails from "./InvoiceLineDetails.vue";

// "Label value" as the cashier reads it (the gap between them is CSS).
const shown = (node: { find: (s: string) => { text: () => string } }) =>
  `${node.find(".invoice-line-detail-label").text()} ${node.find(".invoice-line-detail-value").text()}`;

const mountLine = (line: unknown) => mount(InvoiceLineDetails, { props: { line: line as never }, global: { plugins: [ui] } });

describe("InvoiceLineDetails", () => {
  it("shows the cashier every detail of the sold product", () => {
    const wrapper = mountLine(golden[0]);
    expect(shown(wrapper.find('[data-test="line-code"]'))).toBe("Code TNV551219-16");
    expect(shown(wrapper.find('[data-test="line-barcode"]'))).toBe("Barcode 8000000113603");
    expect(shown(wrapper.find('[data-test="line-size"]'))).toBe("Size 16");
    expect(shown(wrapper.find('[data-test="line-brand"]'))).toBe("Brand Tannav");
    expect(shown(wrapper.find('[data-test="line-productYear"]'))).toBe("Year 2026");
    expect(wrapper.findAll(".invoice-line-detail")).toHaveLength(10);
  });

  it("shows only code and barcode for a bare product", () => {
    const wrapper = mountLine(golden[1]);
    expect(wrapper.findAll(".invoice-line-detail").map(shown)).toEqual(["Code TNV551219", "Barcode 8000000114129"]);
  });

  it("renders nothing for a line with no details", () => {
    const wrapper = mountLine({ code: "", barcode: "" });
    expect(wrapper.find('[data-test="invoice-line-details"]').exists()).toBe(false);
  });
});
