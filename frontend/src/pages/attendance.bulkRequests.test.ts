// @vitest-environment happy-dom
// The Requests menu lets a manager tick several requests and approve or
// reject them together, with one manager password.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { ref } from "vue";
import ui from "@/ui";

globalThis.visualViewport ??= { width: 1024, height: 768, offsetTop: 0, offsetLeft: 0, scale: 1, addEventListener() {}, removeEventListener() {} } as any;

const requests = [
  { id: 1, employeeName: "Sarah M", date: "2026-11-15", status: "pending", action: "add", requestedTime: "09:00", requestedType: "IN" },
  { id: 2, employeeName: "Ali A", date: "2026-11-15", status: "pending", action: "add", requestedTime: "17:00", requestedType: "OUT" },
  { id: 3, employeeName: "Mahdi A", date: "2026-11-14", status: "pending", action: "add", requestedTime: "08:30", requestedType: "IN" },
];
const ApproveAttendanceComplaint = vi.fn(async () => {});
vi.mock("../../wailsjs/go/main/App", () => ({
  AddAttendanceWithManagerApproval: vi.fn(),
  ApproveAttendanceComplaint: (...a: unknown[]) => (ApproveAttendanceComplaint as any)(...a),
  ExportAttendanceMonth: vi.fn(),
  GetAttendanceManagerApprovalEnabled: vi.fn(async () => true),
  ListAttendanceComplaints: vi.fn(async () => requests),
  ListAttendance: vi.fn(async () => []),
  ListSalespersons: vi.fn(async () => []),
  GetAppSettings: vi.fn(async () => ({})),
  SubmitAttendanceComplaint: vi.fn(),
  SyncAttendance: vi.fn(async () => ({})),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: () => () => {} }));
vi.mock("@/lib/salespersonsSync", () => ({ onSalespersonsSynced: () => () => {} }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ isAdmin: true, isLoggedIn: true }) }));
vi.mock("@/composables/useAttendanceStatus", () => ({ useAttendanceStatus: () => ({ status: ref({ reachable: true }) }) }));

import AttendancePage from "./attendance.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.clearAllMocks();
});
const q = <T extends Element = HTMLElement>(sel: string) => document.body.querySelector<T>(sel);
const tick = async (sel: string) => {
  q<HTMLInputElement>(`${sel} input`)!.click();
  await flushPromises();
};

async function openRequests() {
  wrapper = mount(AttendancePage, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  const button = [...document.body.querySelectorAll<HTMLElement>(".k-btn")].find((b) => b.textContent?.includes("Requests"))!;
  button.click();
  await flushPromises();
}

async function confirmWithPassword() {
  const field = [...document.body.querySelectorAll<HTMLInputElement>('input[type="password"]')].at(-1)!;
  field.value = "1234";
  field.dispatchEvent(new Event("input"));
  await flushPromises();
  [...document.body.querySelectorAll<HTMLElement>(".k-btn")].find((b) => b.textContent?.trim() === "Confirm")!.click();
  await flushPromises();
}

describe("Requests menu, several at once", () => {
  it("approves just the ticked requests with one password", async () => {
    await openRequests();
    const approve = q<HTMLButtonElement>('[data-test="requests-approve-selected"]')!;
    expect(approve.disabled).toBe(true);
    await tick('[data-test="request-select-1"]');
    await tick('[data-test="request-select-3"]');
    // The label stays the same, so the menu doesn't change width and move.
    expect(q('[data-test="requests-select-all"]')!.textContent?.trim()).toBe("Select all");
    approve.click();
    await flushPromises();
    expect(document.body.textContent).toContain("Approve 2 requests.");
    await confirmWithPassword();
    expect(ApproveAttendanceComplaint.mock.calls).toEqual([
      [1, true, "1234"],
      [3, true, "1234"],
    ]);
  });

  it("Select all then Reject selected rejects every pending request", async () => {
    await openRequests();
    await tick('[data-test="requests-select-all"]');
    q<HTMLButtonElement>('[data-test="requests-reject-selected"]')!.click();
    await flushPromises();
    expect(document.body.textContent).toContain("Reject 3 requests.");
    await confirmWithPassword();
    expect(ApproveAttendanceComplaint.mock.calls.map((c: any[]) => [c[0], c[1]])).toEqual([
      [1, false],
      [2, false],
      [3, false],
    ]);
  });
});
