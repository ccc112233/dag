export function SystemPage() {
  return (
    <div>
      <div className="page-header">
        <div>
          <h1>系统</h1>
          <p>关于本控制台</p>
        </div>
      </div>
      <div className="panel">
        <p>
          <strong>数采</strong> 工业数据采集平台 MVP
        </p>
        <p>版本：0.1.0</p>
        <p>认证：本地会话（admin / admin）</p>
        <p>部署形态：边缘单机（控制台 + API + 模拟运行时）</p>
      </div>
    </div>
  );
}
