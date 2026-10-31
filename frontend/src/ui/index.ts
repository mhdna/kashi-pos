import type { App, Component } from "vue";
import "./ui.css";
import "./utilities.css";
import KAlert from "./KAlert.vue";
import KApp from "./KApp.vue";
import KAutocomplete from "./KAutocomplete.vue";
import KAvatar from "./KAvatar.vue";
import KBadge from "./KBadge.vue";
import KBtn from "./KBtn.vue";
import KBtnToggle from "./KBtnToggle.vue";
import KCard from "./KCard.vue";
import KCardActions from "./KCardActions.vue";
import KCardSubtitle from "./KCardSubtitle.vue";
import KCardText from "./KCardText.vue";
import KCardTitle from "./KCardTitle.vue";
import KCheckbox from "./KCheckbox.vue";
import KChip from "./KChip.vue";
import KCol from "./KCol.vue";
import KDataTable from "./KDataTable.vue";
import KDialog from "./KDialog.vue";
import KDivider from "./KDivider.vue";
import KExpandTransition from "./KExpandTransition.vue";
import KForm from "./KForm.vue";
import KIcon from "./KIcon";
import KList from "./KList.vue";
import KListItem from "./KListItem.vue";
import KListItemSubtitle from "./KListItemSubtitle.vue";
import KListItemTitle from "./KListItemTitle.vue";
import KListSubheader from "./KListSubheader.vue";
import KMain from "./KMain.vue";
import KMenu from "./KMenu.vue";
import KNavigationDrawer from "./KNavigationDrawer.vue";
import KOverlay from "./KOverlay.vue";
import KProgressCircular from "./KProgressCircular.vue";
import KProgressLinear from "./KProgressLinear.vue";
import KRow from "./KRow.vue";
import KSelect from "./KSelect.vue";
import KSheet from "./KSheet.vue";
import KSlider from "./KSlider.vue";
import KSnackbar from "./KSnackbar.vue";
import KSpacer from "./KSpacer.vue";
import KSwitch from "./KSwitch.vue";
import KTab from "./KTab.vue";
import KTable from "./KTable.vue";
import KTabs from "./KTabs.vue";
import KTextField from "./KTextField.vue";
import KTextarea from "./KTextarea.vue";
import KTooltip from "./KTooltip.vue";
import KVirtualScroll from "./KVirtualScroll.vue";
import KWindow from "./KWindow.vue";
import KWindowItem from "./KWindowItem.vue";

export { useTheme } from "./theme";
export { resolveColor } from "./colors";
export { KTextField, KTextarea, KSelect, KDataTable, KVirtualScroll, KForm, KMenu };

const components = {
  KAlert, KApp, KAutocomplete, KAvatar, KBadge, KBtn, KBtnToggle, KCard, KCardActions, KCardSubtitle, KCardText,
  KCardTitle, KCheckbox, KChip, KCol, KDataTable, KDataTableVirtual: KDataTable, KDialog, KDivider, KExpandTransition,
  KForm, KIcon, KList, KListItem, KListItemSubtitle, KListItemTitle, KListSubheader, KMain, KMenu, KNavigationDrawer,
  KOverlay, KProgressCircular, KProgressLinear, KRow, KSelect, KSheet, KSlider, KSnackbar, KSpacer, KSwitch, KTab,
  KTable, KTabs, KTextField, KTextarea, KTooltip, KVirtualScroll, KWindow, KWindowItem,
};

export default {
  install(app: App) {
    for (const [name, component] of Object.entries(components)) app.component(name, component as Component);
  },
};
