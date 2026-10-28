import type { ReactNode } from 'react'

export default function Modal(props: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <div className="backdrop" onClick={props.onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{props.title}</h3>
        {props.children}
      </div>
    </div>
  )
}
