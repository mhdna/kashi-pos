import vuetify from "eslint-config-vuetify";

export default [
  ...vuetify({
    ts: true,
  }),
  {
    rules: {
      "vue/script-indent": [
        "error",
        2,
        {
          baseIndent: 1,
          switchCase: 1,
        },
      ],
    },
  },
];
