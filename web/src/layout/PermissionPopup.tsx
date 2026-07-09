import type { PermissionRequest } from '../types/permissions'
import { replyPermission } from '../api/permissions'

interface PermissionPopupProps {
  request: PermissionRequest
  projectId: string
  onDismiss: () => void
}

function PermissionPopup({ request, projectId, onDismiss }: PermissionPopupProps) {
  async function handleReply(reply: 'once' | 'always' | 'reject') {
    try {
      await replyPermission(projectId, request.id, { reply })
    } catch {
      /* API error — still dismiss so UI recovers */
    }
    onDismiss()
  }

  return (
    <div className="permission-popup" data-testid="permission-popup">
      <div className="permission-popup__header">
        <span className="permission-popup__title">Permission Required</span>
        <span className="permission-popup__permission-type">
          {request.permission}
        </span>
      </div>
      <div className="permission-popup__body">
        <p className="permission-popup__command">
          <code data-testid="permission-command">
            {request.metadata?.command || request.patterns?.join(', ') || '(unknown command)'}
          </code>
        </p>
      </div>
      <div className="permission-popup__actions">
        <button
          className="permission-popup__btn permission-popup__btn--approve"
          onClick={() => handleReply('once')}
          data-testid="permission-approve-once"
        >
          Approve Once
        </button>
        <button
          className="permission-popup__btn permission-popup__btn--always"
          onClick={() => handleReply('always')}
          data-testid="permission-approve-always"
        >
          Approve Always
        </button>
        <button
          className="permission-popup__btn permission-popup__btn--deny"
          onClick={() => handleReply('reject')}
          data-testid="permission-deny"
        >
          Deny
        </button>
      </div>
    </div>
  )
}

export default PermissionPopup
