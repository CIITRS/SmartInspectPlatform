import '@wangeditor/editor/dist/css/style.css';

import { Editor, Toolbar } from '@wangeditor/editor-for-react';
import { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor';
import { Boot } from '@wangeditor/editor';
import React, { useState, useEffect } from 'react';
import { message } from 'antd';
import { request } from '@umijs/max';

export interface WangEditorProps {
  /** 编辑器内容（HTML格式） */
  value: string;
  /** 内容变化回调 */
  onChange: (value: string) => void;
  /** 占位提示文字 */
  placeholder?: string;
  /** 是否只读模式 */
  readOnly?: boolean;
  /** 编辑器高度 */
  height?: string | number;
  /** 自定义样式 */
  style?: React.CSSProperties;
  /** 类名 */
  className?: string;
}

// 注册自定义菜单 - 上传附件
const UploadAttachmentMenu = {
  key: 'uploadAttachment',
  factory() {
    return {
      title: '上传附件',
      iconSvg: '<svg viewBox="0 0 1024 1024" width="1em" height="1em" fill="currentColor"><path d="M780.8 454.4l-256-256a32 32 0 00-45.2 0l-256 256a32 32 0 0045.2 45.2l218.8-218.8v530.8a32 32 0 1064 0V480.8l218.8 218.8a32 32 0 0045.2-45.2z"></path></svg>',
      tag: 'button',

      getValue(editor: IDomEditor) {
        return '';
      },
      isActive(editor: IDomEditor) {
        return false;
      },
      isDisabled(editor: IDomEditor) {
        return editor.isDisabled();
      },
      exec(editor: IDomEditor, value: string | boolean) {
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = '*/*';
        input.multiple = false;
        input.onchange = async (e) => {
          const file = (e.target as HTMLInputElement).files?.[0];
          if (!file) return;

          try {
            const formData = new FormData();
            formData.append('file', file);

            const res = await request('/api/upload/attachment', {
              method: 'POST',
              data: formData,
            });

            if (res && res.data && res.data.url) {
              const fileName = res.data.name || file.name;
              const fileUrl = res.data.url;
              // 使用 setHtml 或 insertNode 代替 insertHtml
              const currentHtml = editor.getHtml();
              const newHtml = currentHtml + `<p><a href="${fileUrl}" target="_blank" rel="noopener noreferrer">${fileName}</a></p>`;
              editor.setHtml(newHtml);
              message.success('附件上传成功');
            } else {
              message.error('附件上传失败');
            }
          } catch (err) {
            console.error('上传附件出错:', err);
            message.error('附件上传失败');
          }
        };
        input.click();
      },
    };
  },
};

Boot.registerMenu(UploadAttachmentMenu);

const WangEditor: React.FC<WangEditorProps> = ({
  value,
  onChange,
  placeholder = '请输入内容...',
  readOnly = false,
  height = 500,
  style,
  className,
}) => {
  const [editor, setEditor] = useState<IDomEditor | null>(null);
  const [html, setHtml] = useState(value);

  // 同步外部 value 变化
  useEffect(() => {
    setHtml(value);
  }, [value]);

  // 工具栏配置
  const toolbarConfig: Partial<IToolbarConfig> = {
    toolbarKeys: [
      // 撤销重做
      'undo',
      'redo',
      '|',
      // 标题
      'headerSelect',
      '|',
      // 字体
      'fontSize',
      'fontFamily',
      '|',
      // 样式
      'bold',
      'italic',
      'underline',
      'through',
      'color',
      'bgColor',
      '|',
      // 对齐
      'justifyLeft',
      'justifyRight',
      'justifyCenter',
      'justifyJustify',
      '|',
      // 列表
      'bulletedList',
      'numberedList',
      'todo',
      '|',
      // 缩进
      'indent',
      'delIndent',
      '|',
      // 引用、代码
      'quote',
      'codeBlock',
      '|',
      // 链接、图片、视频
      'insertLink',
      'uploadImage',
      'insertVideo',
      'uploadAttachment',
      '|',
      // 表格
      'insertTable',
      '|',
      // 分割线
      'divider',
      '|',
      // 清除格式
      'clearStyle',
    ],
  };

  // 编辑器配置
  const editorConfig: Partial<IEditorConfig> = {
    placeholder,
    readOnly,
    autoFocus: false,
    scroll: true,
    MENU_CONF: {
      // 上传图片配置
      uploadImage: {
        server: '/api/upload/image',
        fieldName: 'file',
        maxFileSize: 5 * 1024 * 1024, // 5MB
        maxNumberOfFiles: 10,
        allowedFileTypes: ['image/*'],
        metaWithUrl: true,
        withCredentials: true,
        timeout: 30 * 1000,
        onSuccess(file: File, res: any) {
          message.success('图片上传成功');
        },
        onFailed(file: File, res: any) {
          message.error('图片上传失败');
        },
        onError(file: File, err: any, res: any) {
          console.error('上传图片出错:', err, res);
          message.error('图片上传出错');
        },
        customInsert(res: any, insertFn: any) {
          const url = res.data?.url || res.url;
          const alt = res.data?.alt || res.alt || '';
          const href = res.data?.href || res.href || url;
          if (url) {
            insertFn(url, alt, href);
          }
        },
      },
      // 上传视频配置
      uploadVideo: {
        server: '/api/upload/video',
        fieldName: 'file',
        maxFileSize: 100 * 1024 * 1024, // 100MB
        maxNumberOfFiles: 5,
        allowedFileTypes: ['video/*'],
        metaWithUrl: true,
        withCredentials: true,
        timeout: 60 * 1000,
        onSuccess(file: File, res: any) {
          message.success('视频上传成功');
        },
        onFailed(file: File, res: any) {
          message.error('视频上传失败');
        },
        onError(file: File, err: any, res: any) {
          console.error('上传视频出错:', err, res);
          message.error('视频上传出错');
        },
      },
    },
  };

  // 编辑器内容变化
  const handleChange = (editor: IDomEditor) => {
    const newHtml = editor.getHtml();
    setHtml(newHtml);
    onChange(newHtml);
  };

  // 销毁编辑器
  useEffect(() => {
    return () => {
      if (editor == null) return;
      editor.destroy();
      setEditor(null);
    };
  }, [editor]);

  const containerStyle: React.CSSProperties = {
    border: '1px solid #ccc',
    position: 'relative',
    zIndex: 1000,
    background: '#fff',
    ...style,
  };

  const editorStyle: React.CSSProperties = {
    height: typeof height === 'number' ? `${height}px` : height,
    overflowY: 'auto',
  };

  return (
    <div className={`hw-wang-editor ${className || ''}`} style={containerStyle}>
      {!readOnly && (
        <Toolbar
          editor={editor}
          defaultConfig={toolbarConfig}
          mode="default"
          style={{ borderBottom: '1px solid #ccc' }}
        />
      )}
      <Editor
        defaultConfig={editorConfig}
        value={html}
        onCreated={setEditor}
        onChange={handleChange}
        mode="default"
        style={editorStyle}
      />
    </div>
  );
};

export default WangEditor;
