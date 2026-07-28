import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Button, Spin, message } from 'antd';
import { LeftOutlined, RightOutlined, DownloadOutlined, ArrowLeftOutlined } from '@ant-design/icons';
import { getReportById } from '@/services/api';

const ReportPreview: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  
  const [loading, setLoading] = useState(true);
  const [report, setReport] = useState<any>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(3); // 默认3页

  useEffect(() => {
    if (id) {
      fetchReportData();
    }
  }, [id]);

  const fetchReportData = async () => {
    try {
      setLoading(true);
      const response = await getReportById(id!);
      setReport(response.data);
    } catch (error) {
      console.error('获取报告数据失败:', error);
      message.error('获取报告数据失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDownloadPDF = async () => {
    try {
      message.info('正在下载...');
      
      const response = await fetch(`/api/reports/${id}/pdf/download`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('下载失败');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `报告_${report?.sampleCode || 'unknown'}_${report?.patientName || 'unknown'}.pdf`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);

      message.success('下载成功');
    } catch (error) {
      console.error('下载失败:', error);
      message.error('下载失败，请重试');
    }
  };

  const renderPage = (pageNum: number) => {
    switch (pageNum) {
      case 1:
        return renderFirstPage();
      case 2:
        return renderSecondPage();
      case 3:
        return renderThirdPage();
      default:
        return null;
    }
  };

  const renderFirstPage = () => (
    <div 
      className="report-page"
      style={{ 
        width: '100%', 
        height: '100%', 
        position: 'relative',
        backgroundImage: `url(/assets/report-bg-1.jpg)`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        padding: '40px',
        boxSizing: 'border-box'
      }}
    >
      <div style={{ 
        position: 'absolute', 
        top: '100px', 
        left: '80px', 
        color: '#333', 
        fontSize: '24px',
        fontWeight: 'bold'
      }}>
        患者姓名: {report?.patientName || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '140px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        样本编号: {report?.sampleCode || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '180px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        性别: {report?.gender || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '220px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        年龄: {report?.patientAge ?? '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '260px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        送检单位: {report?.organization || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '300px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        采样时间: {report?.sampleCollectedAt || report?.time1 || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '340px', 
        left: '80px', 
        color: '#333', 
        fontSize: '18px'
      }}>
        报告类型: {report?.reportType || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        bottom: '100px', 
        left: '80px', 
        color: '#333', 
        fontSize: '16px'
      }}>
        生成日期: {report?.generatedTime || report?.createdAt ? new Date(report?.generatedTime || report?.createdAt).toLocaleDateString() : '-'}
      </div>
    </div>
  );

  const renderSecondPage = () => (
    <div 
      className="report-page"
      style={{ 
        width: '100%', 
        height: '100%', 
        position: 'relative',
        backgroundImage: `url(/assets/report-bg-2.jpg)`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        padding: '40px',
        boxSizing: 'border-box'
      }}
    >
      <div style={{ 
        position: 'absolute', 
        top: '80px', 
        left: '80px', 
        color: '#333', 
        fontSize: '22px',
        fontWeight: 'bold'
      }}>
        检测结果
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '130px', 
        left: '80px', 
        right: '80px',
        color: '#333', 
        fontSize: '16px',
        lineHeight: '2'
      }}>
        <div>计算结果: {report?.calculationResult !== undefined ? report.calculationResult.toFixed(1) : '-'}</div>
        <div style={{ marginTop: '20px' }}>信号值说明: {report?.signalValueExplanation || '-'}</div>
      </div>
    </div>
  );

  const renderThirdPage = () => (
    <div 
      className="report-page"
      style={{ 
        width: '100%', 
        height: '100%', 
        position: 'relative',
        backgroundImage: `url(/assets/report-bg-3.jpg)`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        padding: '40px',
        boxSizing: 'border-box'
      }}
    >
      <div style={{ 
        position: 'absolute', 
        top: '80px', 
        left: '80px', 
        color: '#333', 
        fontSize: '22px',
        fontWeight: 'bold'
      }}>
        结果说明
      </div>
      <div style={{ 
        position: 'absolute', 
        top: '130px', 
        left: '80px', 
        right: '80px',
        color: '#333', 
        fontSize: '16px',
        lineHeight: '2'
      }}>
        {report?.resultExplanation || '-'}
      </div>
      <div style={{ 
        position: 'absolute', 
        bottom: '120px', 
        left: '80px', 
        color: '#333', 
        fontSize: '14px'
      }}>
        <div>检验者: {report?.inspector || '-'}</div>
        <div style={{ marginTop: '10px' }}>报告者: {report?.reporter || '-'}</div>
        <div style={{ marginTop: '10px' }}>审核者: {report?.reviewer || '-'}</div>
      </div>
    </div>
  );

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div style={{ 
      width: '100vw', 
      height: '100vh', 
      background: '#f0f2f5',
      display: 'flex',
      flexDirection: 'column'
    }}>
      {/* 顶部导航栏 */}
      <div style={{ 
        padding: '16px 24px', 
        background: '#fff', 
        borderBottom: '1px solid #e8e8e8',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <Button 
            icon={<ArrowLeftOutlined />} 
            onClick={() => navigate(-1)}
          >
            返回
          </Button>
          <h2 style={{ margin: 0, fontSize: '20px' }}>报告预览</h2>
        </div>
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }}>
          <span>第 {currentPage} / {totalPages} 页</span>
          <Button 
            icon={<LeftOutlined />}
            disabled={currentPage === 1}
            onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
          />
          <Button 
            type="primary"
            icon={<RightOutlined />}
            disabled={currentPage === totalPages}
            onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
          />
          <Button 
            type="primary"
            icon={<DownloadOutlined />}
            onClick={handleDownloadPDF}
          >
            下载PDF
          </Button>
        </div>
      </div>

      {/* 报告内容区域 */}
      <div style={{ 
        flex: 1, 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center',
        padding: '24px',
        overflow: 'auto'
      }}>
        <div style={{ 
          width: '800px', 
          height: '1132px', 
          background: '#fff',
          boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
          position: 'relative'
        }}>
          {renderPage(currentPage)}
        </div>
      </div>
    </div>
  );
};

export default ReportPreview;
