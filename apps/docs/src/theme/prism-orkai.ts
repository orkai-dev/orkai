import type {PrismTheme} from 'prism-react-renderer';

/** Dark — Dev Mode code blocks (sharp, deep ocean) */
export const orkaiDark: PrismTheme = {
  plain: {
    color: '#dce1fb',
    backgroundColor: '#070d1f',
  },
  styles: [
    {types: ['comment', 'prolog', 'doctype', 'cdata'], style: {color: '#899297'}},
    {types: ['punctuation'], style: {color: '#bec8cd'}},
    {types: ['property', 'tag', 'constant', 'symbol', 'deleted'], style: {color: '#ffb4ab'}},
    {types: ['boolean', 'number'], style: {color: '#ffb86f'}},
    {
      types: ['selector', 'attr-name', 'string', 'char', 'builtin', 'inserted'],
      style: {color: '#81d1f0'},
    },
    {types: ['operator', 'entity', 'url'], style: {color: '#bec8cd'}},
    {types: ['atrule', 'attr-value', 'keyword'], style: {color: '#7bd0ff'}},
    {types: ['function', 'class-name'], style: {color: '#d3f1ff'}},
    {types: ['regex', 'important', 'variable'], style: {color: '#b9eaff'}},
  ],
};

/** Light — utility console */
export const orkaiLight: PrismTheme = {
  plain: {
    color: '#0d1c2f',
    backgroundColor: '#ffffff',
  },
  styles: [
    {types: ['comment', 'prolog', 'doctype', 'cdata'], style: {color: '#75777c'}},
    {types: ['punctuation'], style: {color: '#44474b'}},
    {types: ['property', 'tag', 'constant', 'symbol', 'deleted'], style: {color: '#ba1a1a'}},
    {types: ['boolean', 'number'], style: {color: '#ca8a04'}},
    {
      types: ['selector', 'attr-name', 'string', 'char', 'builtin', 'inserted'],
      style: {color: '#006781'},
    },
    {types: ['operator', 'entity', 'url'], style: {color: '#44474b'}},
    {types: ['atrule', 'attr-value', 'keyword'], style: {color: '#0e7490'}},
    {types: ['function', 'class-name'], style: {color: '#0d1c2f'}},
    {types: ['regex', 'important', 'variable'], style: {color: '#16a34a'}},
  ],
};
