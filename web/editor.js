// 只导出页面用到的 CodeMirror 接口，打包后挂在全局 window.CM 上
export { EditorView, keymap, Decoration, WidgetType, placeholder } from '@codemirror/view';
export { EditorState, StateField, StateEffect, Compartment, Transaction } from '@codemirror/state';
export { basicSetup } from 'codemirror';
export { json } from '@codemirror/lang-json';
export { foldEffect, unfoldAll, ensureSyntaxTree, HighlightStyle, syntaxHighlighting, indentUnit } from '@codemirror/language';
export { tags } from '@lezer/highlight';
export { indentWithTab } from '@codemirror/commands';
