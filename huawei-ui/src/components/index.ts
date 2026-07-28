/**
 * 这个文件作为组件的目录
 * 目的是统一管理对外输出的组件，方便分类
 */
/**
 * 布局组件
 */
import Footer from './Footer';
import { AvatarDropdown, AvatarName } from './RightContent/AvatarDropdown';

export { AvatarDropdown, AvatarName, Footer };

/**
 * 富文本编辑器组件
 */
export { default as WangEditor } from './WangEditor';
export type { WangEditorProps } from './WangEditor';
