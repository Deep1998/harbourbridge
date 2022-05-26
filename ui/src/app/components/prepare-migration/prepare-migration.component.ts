import { Component, OnInit } from '@angular/core';

@Component({
  selector: 'app-prepare-migration',
  templateUrl: './prepare-migration.component.html',
  styleUrls: ['./prepare-migration.component.scss']
})
export class PrepareMigrationComponent implements OnInit {

  displayedColumns = [
    'Title',
    'Source',
    'Destination',
  ]
  constructor() { }

  ngOnInit(): void {
  }

}
