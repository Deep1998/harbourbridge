import { Component, OnInit } from '@angular/core';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { MatDialogRef } from '@angular/material/dialog';
import { Datastream } from 'src/app/app.constants';

const MIN_DATASTREAM_TASK_LIMIT = 1 
const MAX_DATASTREAM_TASK_LIMIT = 50

@Component({
  selector: 'app-tune-datastream-form',
  templateUrl: './tune-datastream-form.component.html',
  styleUrls: ['./tune-datastream-form.component.scss']
})
export class TuneDatastreamFormComponent implements OnInit {
  datastreamForm: FormGroup

  constructor(private dialofRef: MatDialogRef<TuneDatastreamFormComponent>) {
    this.datastreamForm = new FormGroup({
      maxConcurrentBackfillTasks: new FormControl('50', [Validators.required, Validators.pattern('^[1-9][0-9]*$'), Validators.min(MIN_DATASTREAM_TASK_LIMIT), Validators.max(MAX_DATASTREAM_TASK_LIMIT)]),
      maxConcurrentCdcTasks: new FormControl('5', [Validators.required, Validators.pattern('^[1-9][0-9]*$'), Validators.min(MIN_DATASTREAM_TASK_LIMIT), Validators.max(MAX_DATASTREAM_TASK_LIMIT)]),
    })
  }

  ngOnInit(): void {
  }

  updateDatastreamDetails() {
    let formValue = this.datastreamForm.value
    localStorage.setItem(Datastream.MaxConcurrentBackfillTasks, formValue.maxConcurrentBackfillTasks)
    localStorage.setItem(Datastream.MaxConcurrentCdcTasks, formValue.maxConcurrentCdcTasks)
    localStorage.setItem(Datastream.IsDatastreamConfigSet, "true")
    this.dialofRef.close()
  }

}
